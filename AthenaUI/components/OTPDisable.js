import { el } from "../lib/dom.js";
import { otpService as customerOtpService } from "../services/otp.service.js";

/**
 * @param {object} props
 * @param {function} props.onSuccess
 * @param {object} [props.otpService] - Der zu verwendende OTP-Service
 */
export function renderOTPDisable(props) {
  const otpService = props.otpService || customerOtpService;

  const { onSuccess } = props;
  const errorMsg = el("p", { className: "error" });

  const codeInput = el("input", {
    type: "text",
    placeholder: "Aktueller 6-stelliger Code",
    required: true,
    maxLength: 6,
  });

  const disableForm = el(
    "form",
    {
      onSubmit: async (e) => {
        e.preventDefault();
        errorMsg.textContent = "";

        if (!confirm("Sind Sie sicher, dass Sie 2FA deaktivieren möchten?")) {
          return;
        }

        try {
          await otpService.disableOTP(codeInput.value);
          alert("2FA erfolgreich deaktiviert!");
          onSuccess(); // Profil-Seite neu laden
        } catch (err) {
          errorMsg.textContent =
            "Deaktivierung fehlgeschlagen. Ist der Code korrekt?";
        }
      },
    },
    [
      el("p", {
        text: "Geben Sie einen aktuellen Code aus Ihrer Authenticator-App ein, um die Deaktivierung zu bestätigen.",
      }),
      el("div", {}, [el("label", { text: "Bestätigungscode:" }), codeInput]),
      el("button", { type: "submit", text: "2FA Deaktivieren" }),
      errorMsg,
    ]
  );

  return el("div", {}, [el("h4", { text: "2FA Deaktivieren" }), disableForm]);
}
