import os
import asyncio
import ollama
import datetime
import re
from pathlib import Path
from textual.app import App, ComposeResult
from textual.containers import Container, ScrollableContainer
from textual.widgets import Footer, Input, Static, Markdown, Label
from textual.binding import Binding
from textual.reactive import reactive

# --- KONFIGURATION ---
DEFAULT_MODEL = "llama3"
SYSTEM_PROMPT_TEMPLATE = """
Du bist ein hochsicherer AI-Assistent (Secure Cockpit).
Datum/Zeit: {time}
Kontext: Du hast Zugriff auf lokale Dateien. Nutze sie für präzise Antworten.
Stil: Technisch, direkt, keine Füllwörter. Code immer in Backticks mit Sprache.
"""

# Sicherheit & Filter
ALLOWED_EXTENSIONS = {
    '.py', '.js', '.ts', '.tsx', '.java', '.cpp', '.c', '.h', '.rs', '.go', 
    '.rb', '.php', '.sh', '.bash', '.md', '.json', '.yaml', '.yml', '.toml', 
    '.xml', '.html', '.css', '.sql', '.dockerfile', '.txt'
}

# Ordner/Dateien die IMMER ignoriert werden
EXCLUDE_DIRS = {'.git', '__pycache__', 'node_modules', 'venv', '.venv', 'dist', 'build', '.idea', '.vscode'}
EXCLUDE_FILES = {'.env', 'id_rsa', 'secrets.yaml', '.DS_Store'}

# Limits
MAX_FILE_SIZE = 5 * 1024 * 1024  # 5MB
MAX_CONTEXT_TOKENS = 8000
MAX_HISTORY_TOKENS = 4000
MAX_FILES_TO_LOAD = 300

class ChatMessage(Static):
    """Widget für eine Nachricht"""
    def __init__(self, role, content, meta_info=""):
        super().__init__()
        self.role = role
        self.content = content
        self.meta_info = meta_info
        self.classes = "msg-ai" if role == "assistant" else "msg-user"

    def compose(self) -> ComposeResult:
        timestamp = datetime.datetime.now().strftime("%H:%M")
        if self.role == "assistant":
            header = f"🤖 AI ({timestamp})"
            if self.meta_info:
                header += f" | {self.meta_info}"
        else:
            header = f"👤 USER ({timestamp})"
            
        yield Label(header, classes="msg-title")
        yield Markdown(self.content)

    def update_content(self, new_content, meta_info=""):
        self.content = new_content
        self.query_one(Markdown).update(new_content)
        if meta_info and self.role == "assistant":
            timestamp = datetime.datetime.now().strftime("%H:%M")
            self.query_one(Label).update(f"🤖 AI ({timestamp}) | {meta_info}")

class SecureAIApp(App):
    """Secure AI Cockpit - Ultimate Edition"""
    
    CSS = """
    Screen { background: #1a1b26; color: #a9b1d6; }
    
    /* Header */
    #header-status {
        dock: top; height: 3; content-align: center middle;
        background: #24283b; color: #7aa2f7; border-bottom: solid #414868; text-style: bold;
    }
    .secure { color: #9ece6a; }
    .insecure { color: #f7768e; }

    /* Chat Area */
    #chat-scroll { height: 1fr; margin: 0 1; scrollbar-gutter: stable; }
    
    .msg-user {
        background: #292e42; margin: 1 2; padding: 1 2;
        border-left: solid #bb9af7 4px; border-radius: 0 8px 8px 0;
        width: 85%; margin-left: 15%;
    }
    
    .msg-ai {
        background: #1f2335; margin: 1 2; padding: 1 2;
        border-left: solid #7aa2f7 4px; border-radius: 0 8px 8px 0;
        width: 90%;
    }
    
    .msg-title { color: #565f89; margin-bottom: 1; text-style: bold; opacity: 0.8; }
    
    /* Input */
    #input-area { 
        dock: bottom; height: auto; min-height: 3; 
        margin: 0 1 1 1; border: solid #565f89; background: #1a1b26; 
    }
    Input { border: none; background: #1a1b26; color: #c0caf5; width: 100%; }
    Input:focus { border: none; }
    
    Markdown > .code_inline { background: #414868; color: #c0caf5; padding: 0 1; }
    """

    BINDINGS = [
        ("ctrl+c", "quit", "Exit"),
        ("ctrl+l", "clear_chat", "Clear"),
        ("ctrl+s", "save_chat", "Save"),
        ("escape", "stop_generation", "Stop"),
    ]

    current_model = reactive(DEFAULT_MODEL)
    is_generating = reactive(False)

    def __init__(self):
        super().__init__()
        self.loaded_files = {} 
        self.pinned_files = set()
        self.context_mode = "smart"
        self.chat_history = []
        self._stop_event = asyncio.Event()

    def on_mount(self):
        self.check_available_models()
        self.query_one("#prompt").focus()

    def check_available_models(self):
        try:
            models = ollama.list().get('models', [])
            model_names = [m['name'] for m in models]
            if self.current_model not in model_names:
                if model_names:
                    self.current_model = model_names[0]
                    self.notify(f"Fallback auf Modell: {self.current_model}", severity="warning")
                else:
                    self.notify("⚠️ Keine Modelle gefunden! Installiere Ollama Modelle.", severity="error")
        except Exception:
            self.notify("⚠️ Keine Verbindung zu Ollama!", severity="error")

    def compose(self) -> ComposeResult:
        vpn_status = os.environ.get("VPN_STATUS", "UNKNOWN")
        icon = "🔒 SECURE" if vpn_status == "SECURE" else "⚠️ OPEN"
        css_class = "secure" if vpn_status == "SECURE" else "insecure"
        
        yield Static(f" COCKPIT ── [{css_class}]{icon}[/] ── Modell: {self.current_model}", id="header-status")
        
        with ScrollableContainer(id="chat-scroll"):
            yield ChatMessage("assistant", 
                f"System bereit (**{self.current_model}**).\n\n"
                "**Wichtige Befehle:**\n"
                "- `/load <path>` Dateien laden (rekursiv)\n"
                "- `/focus <file>` Datei in Kontext pinnen\n"
                "- `/unfocus` Alle Pins entfernen\n"
                "- `/model <name>` Modell wechseln\n"
                "- `/mode <smart|full|summary>` Context-Strategie\n"
                "- `/files` Geladene Dateien anzeigen\n"
                "- `/search <query>` Volltextsuche\n"
                "- `/save` Chat als MD exportieren\n"
                "- `/clear` Reset"
            )
        
        with Container(id="input-area"):
            yield Input(placeholder="Nachricht... (ESC zum Stoppen)", id="prompt")
        yield Footer()

    def watch_current_model(self, val):
        try:
            vpn_status = os.environ.get("VPN_STATUS", "UNKNOWN")
            icon = "🔒 SECURE" if vpn_status == "SECURE" else "⚠️ OPEN"
            css_class = "secure" if vpn_status == "SECURE" else "insecure"
            self.query_one("#header-status").update(
                f" COCKPIT ── [{css_class}]{icon}[/] ── Modell: {val}"
            )
        except: 
            pass

    # --- CORE LOGIC ---

    def estimate_tokens(self, text: str) -> int:
        """Konservative Token-Schätzung"""
        return len(text) // 3.5

    def load_file(self, filepath: Path) -> dict:
        if filepath.name in EXCLUDE_FILES: 
            return None
        if filepath.suffix.lower() not in ALLOWED_EXTENSIONS: 
            return None
        if filepath.stat().st_size > MAX_FILE_SIZE: 
            return {'path': str(filepath), 'error': 'Zu groß (>5MB)'}
        
        try:
            with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
                content = f.read()
                lines = content.split('\n')
                
                # Smart Summary erstellen
                if len(content) > 6000:
                    summary = '\n'.join(lines[:40]) + f"\n... [{len(lines)-60} lines hidden] ...\n" + '\n'.join(lines[-20:])
                else:
                    summary = content

                return {
                    'path': str(filepath), 
                    'name': filepath.name,
                    'content': content, 
                    'summary': summary,
                    'lines': len(lines),
                    'tokens': self.estimate_tokens(content)
                }
        except Exception as e:
            return {'path': str(filepath), 'error': str(e)}

    def load_folder(self, path: Path) -> list:
        files = []
        try:
            if path.is_file():
                res = self.load_file(path)
                if res: files.append(res)
            else:
                for p in path.rglob('*'):
                    # Skip excluded directories
                    if any(part in EXCLUDE_DIRS for part in p.parts): 
                        continue
                    if len(files) >= MAX_FILES_TO_LOAD: 
                        break
                    if p.is_file():
                        res = self.load_file(p)
                        if res: files.append(res)
        except Exception as e:
            return [{'error': str(e)}]
        return files

    def build_context(self, prompt: str) -> str:
        """Intelligenter Context Builder mit Relevanz-Scoring"""
        if not self.loaded_files: 
            return ""
        
        # 1. Relevanz-Scoring
        scored = []
        prompt_lower = prompt.lower()
        
        for path, data in self.loaded_files.items():
            if 'error' in data: 
                continue
            
            score = 0
            
            # Pinned Files haben höchste Priorität
            if data['name'] in self.pinned_files or path in self.pinned_files:
                score = 1000
            
            # Exakter Dateiname-Match
            if data['name'].lower() in prompt_lower: 
                score += 50
            
            # Keyword-Matching im Content
            keywords = [w for w in prompt_lower.split() if len(w) > 4]
            hits = sum(1 for k in keywords if k in data['content'].lower())
            score += hits * 5
            
            scored.append((score, data))

        # 2. Sort & Assemble
        scored.sort(key=lambda x: x[0], reverse=True)
        
        context_parts = ["=== PROVIDED CODE CONTEXT ==="]
        used_tokens = 0
        files_included = 0
        
        for score, data in scored:
            # Entscheidungslogik: Full Content oder Summary?
            is_highly_relevant = score > 40
            use_full = (self.context_mode == "full") or (self.context_mode == "smart" and is_highly_relevant)
            
            content = data['content'] if use_full else data['summary']
            toks = self.estimate_tokens(content)
            
            # Token-Budget Check
            if used_tokens + toks > MAX_CONTEXT_TOKENS:
                # Fallback zu Summary wenn Full zu groß
                if use_full and content != data['summary']:
                    content = data['summary']
                    toks = self.estimate_tokens(content)
                    
                if used_tokens + toks > MAX_CONTEXT_TOKENS:
                    context_parts.append(f"\n[Skipped: {data['name']} - Budget erschöpft]")
                    continue

            context_parts.append(f"\n--- FILE: {data['name']} (Relevanz: {score}) ---\n{content}\n")
            used_tokens += toks
            files_included += 1

        context_parts.append(f"\n=== END CONTEXT ({files_included} Dateien, ~{used_tokens} tokens) ===")
        return "\n".join(context_parts)

    def trim_history(self) -> list:
        """Trimmt History auf Token-Budget"""
        trimmed = []
        total_tokens = 0
        
        # Rückwärts durch History
        for msg in reversed(self.chat_history):
            msg_tokens = self.estimate_tokens(msg['content'])
            if total_tokens + msg_tokens > MAX_HISTORY_TOKENS:
                break
            trimmed.insert(0, msg)
            total_tokens += msg_tokens
        
        return trimmed

    # --- ACTIONS & HANDLERS ---

    async def on_input_submitted(self, event: Input.Submitted):
        prompt = event.value.strip()
        if not prompt: 
            return
        
        self.query_one("#prompt").value = ""
        chat = self.query_one("#chat-scroll")
        
        # Commands
        if prompt.startswith('/'):
            await self.handle_command(prompt)
            return

        # Chat Flow - User Message zur UI hinzufügen
        await chat.mount(ChatMessage("user", prompt))
        chat.scroll_end(animate=False)
        
        # AI Placeholder
        ai_msg = ChatMessage("assistant", "⏳ *Thinking...*")
        await chat.mount(ai_msg)
        chat.scroll_end(animate=False)
        
        # Worker starten
        self.is_generating = True
        self._stop_event.clear()
        self.run_worker(self.stream_response(ai_msg, prompt))

    async def handle_command(self, cmd_str):
        parts = cmd_str.split(' ', 1)
        cmd = parts[0].lower()
        arg = parts[1] if len(parts) > 1 else ""
        chat = self.query_one("#chat-scroll")

        if cmd == '/load':
            self.notify(f"Lade {arg}...", title="System")
            files = self.load_folder(Path(arg).expanduser())
            added = 0
            errors = 0
            
            for f in files:
                if 'content' in f: 
                    self.loaded_files[f['path']] = f
                    added += 1
                elif 'error' in f:
                    errors += 1
                    
            msg = f"📂 **{added} Dateien geladen.** (Total: {len(self.loaded_files)})"
            if errors > 0:
                msg += f"\n⚠️ {errors} Fehler"
            await chat.mount(ChatMessage("assistant", msg))

        elif cmd == '/focus':
            found = [k for k,v in self.loaded_files.items() if arg.lower() in v['name'].lower()]
            if found:
                self.pinned_files.update(found)
                names = ', '.join([self.loaded_files[k]['name'] for k in found])
                self.notify(f"{len(found)} Dateien gepinnt", severity="information")
                await chat.mount(ChatMessage("assistant", f"📌 **Gepinnt:** {names}"))
            else:
                self.notify("Datei nicht gefunden.", severity="warning")

        elif cmd == '/unfocus':
            count = len(self.pinned_files)
            self.pinned_files.clear()
            await chat.mount(ChatMessage("assistant", f"📌 {count} Pins entfernt."))

        elif cmd == '/files':
            if not self.loaded_files:
                await chat.mount(ChatMessage("assistant", "📂 Keine Dateien geladen"))
                return
            
            total_tokens = sum(f.get('tokens', 0) for f in self.loaded_files.values() if 'tokens' in f)
            msg = f"📂 **{len(self.loaded_files)} Dateien** (~{total_tokens:,} tokens)\n\n"
            
            for data in list(self.loaded_files.values())[:20]:  # Max 20 anzeigen
                if 'name' in data:
                    pin = "📌 " if data['name'] in self.pinned_files else ""
                    msg += f"{pin}`{data['name']}` ({data.get('lines', 0)} Zeilen)\n"
            
            if len(self.loaded_files) > 20:
                msg += f"\n... und {len(self.loaded_files)-20} weitere"
                
            await chat.mount(ChatMessage("assistant", msg))

        elif cmd == '/search':
            if not arg:
                self.notify("Nutzung: /search <suchbegriff>", severity="error")
                return
            
            results = []
            for data in self.loaded_files.values():
                if 'content' in data:
                    lines = data['content'].split('\n')
                    matches = [(i+1, line) for i, line in enumerate(lines) 
                              if arg.lower() in line.lower()]
                    if matches:
                        results.append((data['name'], matches[:3]))
            
            if results:
                msg = f"🔍 **Treffer für '{arg}':**\n\n"
                for fname, matches in results[:10]:  # Max 10 Dateien
                    msg += f"**{fname}:**\n"
                    for lno, line in matches:
                        msg += f"  L{lno}: `{line.strip()[:70]}`\n"
                    msg += "\n"
                await chat.mount(ChatMessage("assistant", msg))
            else:
                await chat.mount(ChatMessage("assistant", f"❌ Keine Treffer für '{arg}'"))

        elif cmd == '/model':
            if not arg:
                await chat.mount(ChatMessage("assistant", f"🤖 Aktuelles Modell: **{self.current_model}**"))
            else:
                try:
                    models = [m['name'] for m in ollama.list().get('models', [])]
                    # Partial Match erlauben
                    match = next((m for m in models if arg in m), None)
                    if match:
                        self.current_model = match
                        self.notify(f"Gewechselt zu: {self.current_model}", severity="information")
                        await chat.mount(ChatMessage("assistant", f"🤖 Modell gewechselt: **{match}**"))
                    else:
                        self.notify(f"Modell '{arg}' nicht gefunden.", severity="error")
                        available = ', '.join(models)
                        await chat.mount(ChatMessage("assistant", f"❌ Modell nicht gefunden.\n\n**Verfügbar:** {available}"))
                except Exception as e:
                    self.current_model = arg  # Blind switch
                    await chat.mount(ChatMessage("assistant", f"⚠️ Modell blind gewechselt (Offline?): **{arg}**"))

        elif cmd == '/mode':
            if arg in ['smart', 'full', 'summary']:
                self.context_mode = arg
                self.notify(f"Modus: {arg}", severity="information")
                await chat.mount(ChatMessage("assistant", f"🔧 Context-Modus: **{arg}**"))
            else:
                await chat.mount(ChatMessage("assistant", "❌ Modi: smart, full, summary"))

        elif cmd == '/clear':
            self.chat_history = []
            self.pinned_files.clear()
            await chat.remove_children()
            await chat.mount(ChatMessage("assistant", "🧹 Chat & Memory bereinigt."))
            self.notify("Reset durchgeführt")

        elif cmd == '/save':
            timestamp = datetime.datetime.now().strftime("%Y-%m-%d_%H%M")
            fname = f"session_{timestamp}.md"
            try:
                with open(fname, 'w', encoding='utf-8') as f:
                    f.write(f"# Chat Session - {timestamp}\n\n")
                    for msg in self.chat_history:
                        role = msg['role'].upper()
                        f.write(f"## {role}\n{msg['content']}\n\n")
                self.notify(f"Gespeichert: {fname}")
                await chat.mount(ChatMessage("assistant", f"💾 **Gespeichert:** {fname}"))
            except Exception as e:
                self.notify(f"Fehler: {e}", severity="error")

    async def stream_response(self, widget, prompt):
        """Streaming Logic mit intelligenter History + Context"""
        full_res = ""
        start_time = datetime.datetime.now()
        
        try:
            # 1. Context Building
            context_str = self.build_context(prompt)
            
            # 2. System Prompt
            system_msg = SYSTEM_PROMPT_TEMPLATE.format(time=start_time.strftime("%Y-%m-%d %H:%M"))
            
            # 3. Messages zusammenbauen
            messages = [{'role': 'system', 'content': system_msg}]
            
            # 4. History trimmen und hinzufügen
            trimmed_history = self.trim_history()
            messages.extend(trimmed_history)
            
            # 5. Aktuelle User-Message mit Context
            if context_str:
                final_content = f"{context_str}\n\nUser Question: {prompt}"
            else:
                final_content = prompt
            
            messages.append({'role': 'user', 'content': final_content})

            # 6. Stream
            stream = ollama.chat(model=self.current_model, messages=messages, stream=True)
            
            for chunk in stream:
                if self._stop_event.is_set():
                    full_res += "\n\n[❌ ABGEBROCHEN]"
                    break
                part = chunk['message']['content']
                full_res += part
                widget.update_content(full_res)
                self.query_one("#chat-scroll").scroll_end(animate=False)

            # 7. Zur History hinzufügen (NUR die echten Messages, nicht Context!)
            self.chat_history.append({'role': 'user', 'content': prompt})
            self.chat_history.append({'role': 'assistant', 'content': full_res})
            
            # 8. Performance-Stats
            duration = (datetime.datetime.now() - start_time).total_seconds()
            tokens_in = sum(self.estimate_tokens(m['content']) for m in messages)
            tokens_out = self.estimate_tokens(full_res)
            
            meta = f"{duration:.1f}s | In: ~{tokens_in} | Out: ~{tokens_out} toks"
            widget.update_content(full_res, meta_info=meta)

        except Exception as e:
            widget.update_content(f"❌ Error: {e}")
            self.notify("Generation Error", severity="error")
        finally:
            self.is_generating = False

    def action_stop_generation(self):
        if self.is_generating:
            self._stop_event.set()
            self.notify("Stop signal sent", severity="warning")

if __name__ == "__main__":
    app = SecureAIApp()
    app.run()