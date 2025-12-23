import { el, clear } from "../lib/dom.js";
import { navigate } from "../router/index.js";
import { projectStore } from "../store/project.store.js";
import { adminService } from "../services/admin.service.js";
import { renderOTPSetup } from "../components/OTPSetup.js";
import { renderOTPDisable } from "../components/OTPDisable.js";

/**
 * Rendert die Admin-Profil-Ansicht (für globales 2FA).
 * @param {HTMLElement} root
 */
export function renderAdminProfileView(root) {
  clear(root);

  projectStore.setCurrentProjectID(null);

  const profileContainer = el("div", {});
  const managementContainer = el("div", {});

  const view = el("div", {}, [
    el("a", {
      href: "#/dashboard",
      text: "← Zurück zum Dashboard",
      onClick: (e) => {
        e.preventDefault();
        navigate("#/dashboard");
      },
    }),
    el("h2", { text: "Admin-Einstellungen" }),
    el("p", {
      text: "Verwalten Sie hier Ihre globale Zwei-Faktor-Authentifizierung (2FA) für den Admin-Zugang.",
    }),
    profileContainer,
    el("hr"),
    managementContainer,
  ]);

  root.appendChild(view);

  // --- Lade-Logik ---

  async function loadAdminProfile() {
    profileContainer.textContent = "Lade Admin-Profil...";
    try {
      // 1. Profil laden (verwendet adminService)
      const profile = await adminService.getAdminProfile();

      clear(profileContainer);
      profileContainer.append(
        el("p", {}, [
          el("strong", { text: "E-Mail: " }),
          el("span", { text: profile.email }),
        ]),
        el("p", {}, [
          el("strong", { text: "Globaler 2FA-Status: " }),
          el("span", { text: profile.otp_enabled ? "Aktiv" : "Inaktiv" }),
        ])
      );

      // 2. 2FA-Management rendern
      clear(managementContainer);
      if (profile.otp_enabled) {
        const adminOtpDisableWrapper = {
          disableOTP: adminService.disableGlobalOTP,
        };
        const disableComponent = renderOTPDisable({
          onSuccess: reloadView,
          otpService: adminOtpDisableWrapper,
        });
        managementContainer.appendChild(disableComponent);
      } else {
        const adminOtpSetupWrapper = {
          setupOTP: adminService.setupGlobalOTP,
          verifyOTP: adminService.verifyGlobalOTP,
        };
        const setupComponent = renderOTPSetup({
          onSuccess: reloadView,
          otpService: adminOtpSetupWrapper, // Übergibt den Wrapper
        });
        managementContainer.appendChild(setupComponent);
      }
    } catch (err) {
      profileContainer.textContent =
        "Fehler beim Laden des Admin-Profils: " +
        (err.response?.data?.error || err.message);
    }
  }

  function reloadView() {
    renderAdminProfileView(root);
  }

  // --- Initialisierung ---
  loadAdminProfile();
}
