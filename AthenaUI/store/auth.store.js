import { projectStore } from "./project.store.js";

// Helfer-Funktion zum Parsen des JWT-Payloads (Base64-Dekodierung)
function parseJwt(token) {
  try {
    const base64Url = token.split(".")[1];
    const base64 = base64Url.replace(/-/g, "+").replace(/_/g, "/");
    const jsonPayload = decodeURIComponent(
      atob(base64)
        .split("")
        .map(function (c) {
          return "%" + ("00" + c.charCodeAt(0).toString(16)).slice(-2);
        })
        .join("")
    );
    return JSON.parse(jsonPayload);
  } catch (e) {
    console.error("Fehler beim Parsen des JWT:", e);
    return null;
  }
}

// Zustand speichert jetzt den Token UND die dekodierten Claims
let _state = {
  token: localStorage.getItem("token") || null,
  claims: null,
  force_2fa_setup_required: false,
};

// Initiales Laden
if (_state.token) {
  _state.claims = parseJwt(_state.token);
}

let _listeners = [];

export const authStore = {
  notify() {
    _listeners.forEach((listener) => listener(_state));
  },
  subscribe(listener) {
    _listeners.push(listener);
  },

  /**
   * Setzt den Token und parst die Claims.
   * @param {string} token
   * @param {object} [loginResponseData] - Optionale Daten aus der Login-Antwort
   */
  setAuth(token, loginResponseData = {}) {
    _state.token = token;
    _state.claims = parseJwt(token);
    _state.force_2fa_setup_required =
      loginResponseData.force_2fa_setup_required || false;
    localStorage.setItem("token", token);
    projectStore.setCurrentProjectID(loginResponseData.project_id || null);
    this.notify();
  },

  clearAuth() {
    _state.token = null;
    _state.claims = null;
    _state.force_2fa_setup_required = false;
    localStorage.removeItem("token");

    projectStore.setCurrentProjectID(null);

    this.notify();
  },

  getState() {
    return _state;
  },
  isAuthenticated() {
    return !!_state.token;
  },

  isAdmin() {
    return !!_state.claims?.is_admin;
  },

  getUserID() {
    return _state.claims?.user_id || null;
  },

  isGraceLogin() {
    return !!_state.token && _state.force_2fa_setup_required;
  },
};
