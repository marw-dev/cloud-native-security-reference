import { el, clear } from "../lib/dom.js";
import { navigate } from "../router/index.js";
import { projectStore } from "../store/project.store.js";
import { userService } from "../services/user.service.js";
import { projectService } from "../services/project.service.js";
import { renderOTPSetup } from "../components/OTPSetup.js";
import { renderOTPDisable } from "../components/OTPDisable.js";
import { otpService as customerOtpService } from "../services/otp.service.js";

/**
 * Rendert die Profil-Ansicht.
 * @param {HTMLElement} root
 */
export function renderProfileView(root) {
  clear(root);

  let currentProjectID = projectStore.getCurrentProjectID();

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
    el("h2", { text: "Mein Profil" }),
    profileContainer,
    el("hr"),
    managementContainer,
  ]);

  root.appendChild(view);

  // --- Lade-Logik ---

  async function loadProfile() {
    try {
      // 1. Profil laden
      const profile = await userService.getProfile();

      clear(profileContainer);
      profileContainer.append(
        el("p", {}, [
          el("strong", { text: "E-Mail: " }),
          el("span", { text: profile.email }),
        ]),
        el("p", {}, [
          el("strong", { text: "2FA-Status: " }),
          el("span", { text: profile.otp_enabled ? "Aktiv" : "Inaktiv" }),
        ])
      );

      // 2. 2FA-Management rendern
      clear(managementContainer);
      if (profile.otp_enabled) {
        managementContainer.appendChild(
          renderOTPDisable({ onSuccess: reloadView })
        );
      } else {
        managementContainer.appendChild(
          renderOTPSetup({
            onSuccess: reloadView,
            otpService: customerOtpService,
          })
        );
      }
    } catch (err) {
      profileContainer.textContent =
        "Fehler beim Laden des Profils: " +
        (err.response?.data?.error || err.message);
      if (err.response?.status === 400) {
        // (X-Project-ID fehlt)
        profileContainer.textContent +=
          " (Stellen Sie sicher, dass Sie mindestens einem Projekt zugewiesen sind.)";
      }
    }
  }

  function reloadView() {
    renderProfileView(root);
  }

  // --- Initialisierung ---

  if (currentProjectID) {
    profileContainer.textContent = "Lade Profil...";
    loadProfile();
  } else {
    profileContainer.textContent = "Lade Projekt-Kontext...";

    projectService
      .getProjects()
      .then((projects) => {
        if (projects.length > 0) {
          projectStore.setCurrentProjectID(projects[0].id);
          loadProfile();
        } else {
          profileContainer.textContent =
            "Sie müssen Mitglied in mindestens einem Projekt sein, um Ihr Profil zu verwalten.";
        }
      })
      .catch((err) => {
        profileContainer.textContent =
          "Fehler beim Laden der Projekte: " + err.message;
      });
  }
}
