import { clear } from "../lib/dom.js";
import { authService } from "../services/auth.service.js";
import { renderRegisterForm } from "../components/RegisterForm.js";
import { navigate } from "../router/index.js";

/**
 * Rendert die Registrierungs-Ansicht
 * @param {HTMLElement} root
 */
export function renderRegisterView(root) {
  clear(root);

  let state = {
    error: "",
    success: "",
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    const email = e.target.email.value;
    const password = e.target.password.value;
    const confirmPassword = e.target.confirmPassword.value;

    const registrationSecret = e.target.registrationSecret.value;

    // Client-seitige Validierung
    if (password !== confirmPassword) {
      state.error = "Die Passwörter stimmen nicht überein.";
      rerender();
      return;
    }
    if (password.length < 8) {
      state.error = "Passwort muss mindestens 8 Zeichen lang sein.";
      rerender();
      return;
    }

    try {
      state.error = "";
      state.success = "Registrierung wird durchgeführt...";
      rerender();

      await authService.register(email, password, registrationSecret);

      state.success =
        "Registrierung erfolgreich! Du wirst zum Login weitergeleitet...";
      rerender();

      setTimeout(() => {
        navigate("#/login");
      }, 2000);
    } catch (err) {
      state.success = "";
      state.error =
        err.response?.data?.error || "Registrierung fehlgeschlagen.";
      rerender();
    }
  };

  function rerender() {
    clear(root);
    root.appendChild(
      renderRegisterForm({
        onSubmit: handleSubmit,
        error: state.error,
        success: state.success,
      })
    );
  }

  rerender();
}
