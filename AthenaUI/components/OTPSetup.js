import { el, clear } from "../lib/dom.js";
import { otpService as customerOtpService } from "../services/otp.service.js";
import { authStore } from "../store/auth.store.js";

/**
 * @param {object} props
 * @param {function} props.onSuccess
 * @param {object} [props.otpService] - Der zu verwendende OTP-Service (admin oder kunde)
 */
export function renderOTPSetup(props) {
  const otpService = props.otpService || customerOtpService;

  const { onSuccess } = props;
  const container = el("div", {});
  const errorMsg = el("p", { className: "error" });

  const showSetupDetails = (setupData) => {
    clear(container);

    const qrCodeImg = el("img", {});
    // Der QR-Code kommt als Base64-PNG vom Backend
    qrCodeImg.src = `data:image/png;base64,${setupData.qr_code}`;

    const secretKeyEl = el("code", { text: setupData.secret });
    const verifyInput = el("input", {
      type: "text",
      placeholder: "6-stelliger Code",
      required: true,
      maxLength: 6,
    });
    const verifyForm = el(
      "form",
      {
        onSubmit: async (e) => {
          e.preventDefault();
          errorMsg.textContent = "";
          try {
            const loginResponse = await otpService.verifyOTP(verifyInput.value);
            authStore.setAuth(loginResponse.access_token, loginResponse);
            alert("2FA erfolgreich aktiviert!");
            onSuccess(); // Profil-Seite neu laden
          } catch (err) {
            errorMsg.textContent =
              "Verifizierung fehlgeschlagen. Ist der Code korrekt?";
          }
        },
      },
      [
        el("label", { text: "Bestätigungscode eingeben:" }),
        verifyInput,
        el("button", { type: "submit", text: "Aktivierung abschließen" }),
      ]
    );

    container.append(
      el("h4", { text: "2FA-Aktivierung Schritt 2: Bestätigen" }),
      el("p", {
        text: "1. Scannen Sie den QR-Code mit Ihrer Authenticator-App:",
      }),
      qrCodeImg,
      el("p", { text: "2. Oder geben Sie den Schlüssel manuell ein:" }),
      secretKeyEl,
      el("p", {
        text: "3. Geben Sie den generierten Code ein, um die Einrichtung zu bestätigen:",
      }),
      verifyForm,
      errorMsg
    );
  };

  const startButton = el("button", {
    text: "Zwei-Faktor-Authentifizierung (2FA) jetzt einrichten",
    onClick: async () => {
      startButton.textContent = "Lade Setup...";
      startButton.disabled = true;
      errorMsg.textContent = "";
      try {
        const setupData = await otpService.setupOTP();
        showSetupDetails(setupData);
      } catch (err) {
        errorMsg.textContent = "Fehler beim Starten des 2FA-Setups.";
        startButton.textContent = "2FA jetzt einrichten";
        startButton.disabled = false;
      }
    },
  });

  container.append(
    el("h4", { text: "2FA-Aktivierung Schritt 1: Starten" }),
    el("p", { text: "Schützen Sie Ihr Konto zusätzlich." }),
    startButton,
    errorMsg
  );

  return container;
}
