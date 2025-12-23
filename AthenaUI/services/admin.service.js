import apiClient from "./api.js";

/**
 * Service für globale Admin-Aktionen (z.B. globales 2FA).
 * Diese Endpunkte benötigen KEINE X-Project-ID.
 */
export const adminService = {
  /**
   * Ruft die globalen Profileinstellungen für den Admin ab
   */
  async getAdminProfile() {
    try {
      apiClient.defaults.headers.common["X-Project-ID"] = null;
      const response = await apiClient.get("/admin/otp/profile");
      return response.data;
    } catch (err) {
      console.error("Fehler beim Abrufen des Admin-Profils:", err);
      throw err;
    }
  },
  /**
   * Startet den globalen 2FA-Setup-Prozess.
   * (POST /admin/otp/setup)
   */
  async setupGlobalOTP() {
    try {
      apiClient.defaults.headers.common["X-Project-ID"] = null;
      const response = await apiClient.post("/admin/otp/setup");
      return response.data;
    } catch (err) {
      console.error("Fehler beim Starten des globalen OTP-Setups:", err);
      throw err;
    }
  },

  /**
   * Bestätigt und aktiviert globales 2FA.
   * (POST /admin/otp/verify)
   * @param {string} otpCode
   */
  async verifyGlobalOTP(otpCode) {
    try {
      apiClient.defaults.headers.common["X-Project-ID"] = null;
      const response = await apiClient.post("/admin/otp/verify", {
        otp_code: otpCode,
      });

      return response.data;
    } catch (err) {
      console.error("Fehler beim Verifizieren des globalen OTP-Codes:", err);
      throw err;
    }
  },

  /**
   * Deaktiviert globales 2FA.
   * (POST /admin/otp/disable)
   * @param {string} otpCode
   */
  async disableGlobalOTP(otpCode) {
    try {
      apiClient.defaults.headers.common["X-Project-ID"] = null;
      await apiClient.post("/admin/otp/disable", { otp_code: otpCode });
    } catch (err) {
      console.error("Fehler beim Deaktivieren des globalen OTP:", err);
      throw err;
    }
  },
};
