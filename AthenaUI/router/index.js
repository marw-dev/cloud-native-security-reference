import { authStore } from "../store/auth.store.js";
import { renderLoginView } from "../views/LoginView.js";
import { renderRegisterView } from "../views/RegisterView.js";
import { renderDashboardView } from "../views/DashboardView.js";
import { renderProjectDetailView } from "../views/ProjectDetailView.js";
import { renderProfileView } from "../views/ProfileView.js";
// NEUER IMPORT:
import { renderAdminProfileView } from "../views/AdminProfileView.js";

const root = document.getElementById("app-root");

// 1. Routen-Definition
const routes = {
  "#/login": { view: renderLoginView, private: false },
  "#/register": { view: renderRegisterView, private: false },
  "#/dashboard": { view: renderDashboardView, private: true },
  "#/project/:id": { view: renderProjectDetailView, private: true },
  "#/profile": { view: renderProfileView, private: true },
  "#/admin/profile": {
    view: renderAdminProfileView,
    private: true,
    admin: true,
  },
};

/**
 * Wechselt die Ansicht basierend auf dem Hash und dem Login-Status.
 */
function handleRouteChange() {
  if (!root) return;

  const hash = window.location.hash || "#/login";
  let param = null;
  let pathDefinition = hash;

  // Projekt-Route
  if (hash.startsWith("#/project/")) {
    pathDefinition = "#/project/:id";
    param = hash.substring("#/project/".length);
  }

  // Parameterlose Routen
  if (hash === "#/profile") pathDefinition = "#/profile";
  if (hash === "#/admin/profile") pathDefinition = "#/admin/profile";

  const route = routes[pathDefinition];
  if (!route) {
    navigate("#/login");
    return;
  }

  if (authStore.isAuthenticated()) {
    // Prüfe auf 2FA-Zwang
    if (authStore.isGraceLogin() && !hash.startsWith("#/profile")) {
      // Wenn der Benutzer im Grace-Modus ist, aber NICHT zur Profilseite
      // will, zwingen wir ihn dorthin.
      navigate("#/profile");
      return;
    }

    if (route.admin && !authStore.isAdmin()) {
      navigate("#/dashboard"); // Kein Admin? -> Zum Dashboard
      return;
    }

    if (route.private) {
      route.view(root, param);
    } else {
      if (!authStore.isGraceLogin()) {
        navigate("#/dashboard");
      } else {
        navigate("#/profile");
      }
    }
  } else {
    if (route.private) {
      navigate("#/login");
    } else {
      route.view(root);
    }
  }
}

/**
 * Globale Navigationsfunktion (jetzt exportiert).
 * @param {string} path (z.B. '#/dashboard')
 */
export function navigate(path) {
  window.location.hash = path;
}

/**
 * Initialisiert den Router.
 */
export function initRouter() {
  window.addEventListener("hashchange", handleRouteChange);
  authStore.subscribe(handleRouteChange);
  handleRouteChange();
}
