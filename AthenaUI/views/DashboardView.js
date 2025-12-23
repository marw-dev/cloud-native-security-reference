import { el, clear } from "../lib/dom.js";
import { authService } from "../services/auth.service.js";
import { renderCreateProjectForm } from "../components/CreateProjectForm.js";
import { renderProjectList } from "../components/ProjectList.js";
import { projectStore } from "../store/project.store.js";
import { navigate } from "../router/index.js";
import { authStore } from "../store/auth.store.js";

/**
 * Rendert die Dashboard-Ansicht
 * @param {HTMLElement} root
 */
export function renderDashboardView(root) {
  clear(root);
  projectStore.setCurrentProjectID(null);

  const projectList = renderProjectList();
  const createProjectForm = renderCreateProjectForm({
    onProjectCreated: () => {
      projectList.reload();
    },
  });

  const profileButton = authStore.isAdmin()
    ? el("button", {
        text: "Admin-Einstellungen (Global 2FA)",
        onClick: () => navigate("#/admin/profile"),
      })
    : el("button", {
        text: "Mein Profil (Projekt 2FA)",
        onClick: () => navigate("#/profile"),
      });

  const view = el(
    "div",
    {},
    [
      el("h2", { text: "Dashboard" }),

      el(
        "div",
        { className: "button-group" },
        [
          profileButton,
          el("button", {
            text: "Ausloggen",
            onClick: () => authService.logout(),
          }),
        ].filter(Boolean)
      ), // .filter(Boolean) entfernt 'null'-Einträge

      el("hr"),
      el("h3", { text: "Meine Projekte" }),
      projectList.element,

      authStore.isAdmin() ? el("hr") : null,
      authStore.isAdmin() ? createProjectForm : null,
    ].filter(Boolean)
  );

  root.appendChild(view);
}
