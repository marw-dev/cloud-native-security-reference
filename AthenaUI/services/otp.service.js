import apiClient from "./api.js";

export const otpService = {
  /**
   * Startet den 2FA-Setup-Prozess.
   * (POST /auth/otp/setup)
   */
  async setupOTP() {
    try {
      // Benötigt X-Project-ID, wird von apiClient hinzugefügt
      const response = await apiClient.post("/auth/otp/setup");
      return response.data; // Gibt { secret, qr_code, auth_url } zurück
    } catch (err) {
      console.error("Fehler beim Starten des OTP-Setups:", err);
      throw err;
    }
  },

  /**
   * Bestätigt und aktiviert 2FA mit dem Code.
   * (POST /auth/otp/verify)
   * @param {string} otpCode
   */
  async verifyOTP(otpCode) {
    try {
      const response = await apiClient.post("/auth/otp/verify", {
        otp_code: otpCode,
      });
      return response.data;
    } catch (err) {
      console.error("Fehler beim Verifizieren des OTP-Codes:", err);
      throw err;
    }
  },

  /**
   * Deaktiviert 2FA mit einem Bestätigungs-Code.
   * (POST /auth/otp/disable)
   * @param {string} otpCode
   */
  async disableOTP(otpCode) {
    try {
      await apiClient.post("/auth/otp/disable", { otp_code: otpCode });
    } catch (err) {
      console.error("Fehler beim Deaktivieren von OTP:", err);
      throw err;
    }
  },
};
