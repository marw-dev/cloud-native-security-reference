import { el, clear } from "../lib/dom.js";
import { authService } from "../services/auth.service.js";
import { renderLoginForm } from "../components/LoginForm.js";
import { renderOTPForm } from "../components/OTPForm.js";

/**
 * Rendert die Login-Ansicht
 * @param {HTMLElement} root
 */
export function renderLoginView(root) {
  clear(root);

  let viewState = "credentials";
  let errorMessage = "";

  let storedEmail = "";
  let storedPassword = "";
  let tempProjectID = null;

  // --- Handler für E-Mail & Passwort ---
  const handleCredentialsSubmit = async (e) => {
    e.preventDefault();
    const email = e.target.email.value;
    const password = e.target.password.value;

    errorMessage = "Melde an...";
    storedEmail = email;
    storedPassword = password;
    rerender();

    try {
      const response = await authService.login(email, password);

      if (response.global_otp_required) {
        errorMessage = "Globales Admin 2FA erforderlich.";
        viewState = "otp"; // Zustand auf 2FA-Eingabe ändern
        rerender();
      } else if (response.otp_required) {
        errorMessage = "Projekt-2FA erforderlich.";
        viewState = "project_otp";
        tempProjectID = response.project_id;
        rerender();
      }
    } catch (err) {
      errorMessage = err.response?.data?.error || "Login fehlgeschlagen";
      rerender();
    }
  };

  // --- Handler für Schritt 2: Admin OTP ---
  const handleOtpSubmit = async (e) => {
    e.preventDefault();
    const otpCode = e.target.otp.value;
    errorMessage = "Prüfe Code...";
    rerender();

    try {
      await authService.loginAdminOTP(storedEmail, storedPassword, otpCode);
    } catch (err) {
      errorMessage =
        err.response?.data?.error || "Ungültige Anmeldedaten oder 2FA-Code.";
      viewState = "otp"; // Im 2FA-Modus bleiben
      rerender();
    }
  };

  // --- Handler Projekt OTP (Kunde) ---
  const handleProjectOtpSubmit = async (e) => {
    e.preventDefault();
    const otpCode = e.target.otp.value;
    errorMessage = "Prüfe Code...";
    rerender();

    try {
      await authService.loginProjectOTP(
        storedEmail,
        storedPassword,
        otpCode,
        tempProjectID
      );
    } catch (err) {
      errorMessage =
        err.response?.data?.error || "Ungültige Anmeldedaten oder 2FA-Code.";
      viewState = "project_otp";
      rerender();
    }
  };

  // --- Handler zum Abbrechen ---
  const handleCancelOtp = () => {
    viewState = "credentials";
    errorMessage = "";
    storedEmail = "";
    storedPassword = "";
    tempProjectID = null;
    rerender();
  };

  function rerender() {
    clear(root);

    if (viewState === "otp") {
      // Zeige das 2FA-Formular
      root.appendChild(
        renderOTPForm({
          onSubmit: handleOtpSubmit,
          onCancel: handleCancelOtp,
          error: errorMessage,
        })
      );
    } else if (viewState === "project_otp") {
      root.appendChild(
        renderOTPForm({
          onSubmit: handleProjectOtpSubmit,
          onCancel: handleCancelOtp,
          error: errorMessage,
        })
      );
    } else {
      // Zeige das Standard-Login-Formular
      root.appendChild(
        renderLoginForm({
          onSubmit: handleCredentialsSubmit,
          error: errorMessage,
        })
      );
    }
  }

  rerender();
}
