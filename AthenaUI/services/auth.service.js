import apiClient from "./api.js";
import { authStore } from "../store/auth.store.js";
import { projectStore } from "../store/project.store.js";

export const authService = {
  /**
   * Führt den ersten Schritt des Logins durch (E-Mail/Passwort).
   * Gibt die Antwort-Daten zurück (kann {access_token} oder {global_otp_required} sein).
   */
  async login(email, password) {
    projectStore.setCurrentProjectID(null);

    try {
      // Sende NUR E-Mail und Passwort
      const response = await apiClient.post("/auth/login", { email, password });

      if (response.data.access_token) {
        authStore.setAuth(response.data.access_token, response.data);
      } else if (response.data.grace_token) {
        authStore.setAuth(response.data.grace_token, response.data);
      }

      return response.data;
    } catch (err) {
      projectStore.setCurrentProjectID(null);
      throw err;
    }
  },

  /**
   * Führt den zweiten Schritt des Admin-Logins (mit 2FA) durch.
   * @param {string} email
   * @param {string} password
   * @param {string} otpCode
   */
  async loginAdminOTP(email, password, otpCode) {
    projectStore.setCurrentProjectID(null); // Sicherstellen, dass kein Kontext gesendet wird
    const response = await apiClient.post("/auth/login/admin-otp", {
      email,
      password,
      otp_code: otpCode,
    });

    const token = response.data.access_token;
    authStore.setAuth(token, response.data); // Setzt Token, löscht 2FA-Status
    return response.data;
  },

  /**
   * Führt den zweiten Schritt des Projekt-Logins (PROJEKT 2FA) durch.
   */
  async loginProjectOTP(email, password, otpCode, projectID) {
    projectStore.setCurrentProjectID(null); // Sende keinen Header
    const response = await apiClient.post("/auth/login/otp", {
      email,
      password,
      otp_code: otpCode,
      project_id: projectID, // Sende die Projekt-ID im BODY
    });

    const token = response.data.access_token;
    // setAuth speichert den Token UND die project_id aus der Antwort
    authStore.setAuth(token, response.data);
    return response.data;
  },

  async register(email, password, registrationSecret) {
    projectStore.setCurrentProjectID(null);
    await apiClient.post("/auth/register", {
      email,
      password,
      registration_secret: registrationSecret,
    });
  },

  logout() {
    authStore.clearAuth();
  },
};
