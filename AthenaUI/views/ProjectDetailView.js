import { el, clear } from "../lib/dom.js";
import { projectStore } from "../store/project.store.js";
import { projectService } from "../services/project.service.js";
import { navigate } from "../router/index.js";
import { renderRouteList } from "../components/RouteList.js";
import { renderRouteEditor } from "../components/RouteEditor.js";
import { renderProjectSettingsForm } from "../components/ProjectSettingsForm.js";
import { renderUserManagement } from "../components/UserManagment.js";
import { userService } from "../services/user.service.js";
import { authStore } from "../store/auth.store.js";

/**
 * Rendert die Projekt-Detailansicht.
 * @param {HTMLElement} root
 * @param {string} projectID
 */
export function renderProjectDetailView(root, projectID) {
  clear(root);
  projectStore.setCurrentProjectID(projectID);

  // --- Zustand für den Editor ---
  let routeToEdit = null;
  let routeListInstance = null; // Referenz auf die Routenliste

  // Container für den Editor (damit wir ihn leicht neu rendern können)
  const routeEditorContainer = el("div", {
    className: "route-editor-container",
  });

  /**
   * Wird aufgerufen, wenn "Bearbeiten" in der Liste geklickt wird.
   * @param {object} route
   */
  function handleEditRoute(route) {
    routeToEdit = route;
    rerenderEditor();
    window.scrollTo(0, 0);
  }

  /**
   * Wird aufgerufen, wenn der Editor "Abbrechen" klickt oder "Speichern" fertig ist.
   */
  function handleCloseEditor() {
    routeToEdit = null;
    rerenderEditor();
  }

  /**
   * Wird aufgerufen, NACHDEM der Editor gespeichert hat.
   */
  function handleRouteSaved() {
    handleCloseEditor();
    if (routeListInstance) {
      routeListInstance.reload();
    }
  }

  /**
   * Rendert den Editor-Container
   */
  function rerenderEditor() {
    clear(routeEditorContainer);
    const editor = renderRouteEditor({
      projectID: projectID,
      routeToEdit: routeToEdit,
      onRouteSaved: handleRouteSaved,
      onCancel: handleCloseEditor,
    });
    routeEditorContainer.appendChild(editor);
  }

  // --- Initiales Layout ---
  const backLink = el("a", {
    href: "#/dashboard",
    text: "← Zurück zur Projektübersicht",
    onClick: (e) => {
      e.preventDefault();
      navigate("#/dashboard");
    },
  });

  const projectTitle = el("h2", { text: "Lade Projekt..." });
  const tabsContainer = el("div", { className: "tabs-container" });
  const contentContainer = el("div", { className: "tab-content" });

  const view = el("div", {}, [
    backLink,
    projectTitle,
    el("pre", { text: `Projekt-ID: ${projectID}` }),
    tabsContainer,
    el("hr"),
    contentContainer,
  ]);

  root.appendChild(view);

  // --- Daten laden ---
  projectService
    .getProjectDetails(projectID)
    .then((project) => {
      projectTitle.textContent = `Projekt: ${project.name}`;

      // Initialisiere die Komponenten
      routeListInstance = renderRouteList(projectID, handleEditRoute);

      rerenderEditor();

      const settingsForm = renderProjectSettingsForm({
        projectID: projectID,
        project: project,
        onSettingsUpdated: (newName) => {
          projectTitle.textContent = `Projekt: ${newName}`;
        },
      });

      const userManagement = authStore.isAdmin()
        ? renderUserManagement(projectID, userService)
        : null;

      // Tab-Inhalte
      const tabContents = {
        routes: el("div", {}, [
          routeEditorContainer,
          el("hr"),
          routeListInstance.element,
        ]),
        settings: el("div", {}, [settingsForm]),
        users: userManagement, // Ist 'null', wenn kein Admin
      };

      // Tab-Buttons
      const routesTab = el("button", { text: "Routen" });
      const settingsTab = el("button", { text: "Einstellungen" });
      const usersTab = authStore.isAdmin()
        ? el("button", { text: "Benutzer" })
        : null;

      // Array der gültigen Tabs
      const tabs = [routesTab, settingsTab, usersTab].filter(Boolean);

      const showTab = (content, activeTab) => {
        clear(contentContainer);
        contentContainer.appendChild(content);

        // Setze den "active"-Status für die Buttons
        tabs.forEach((tab) => tab.classList.remove("active"));
        if (activeTab) {
          activeTab.classList.add("active");
        }
      };

      routesTab.onclick = () => showTab(tabContents.routes, routesTab);
      settingsTab.onclick = () => showTab(tabContents.settings, settingsTab);
      if (usersTab) {
        usersTab.onclick = () => showTab(tabContents.users, usersTab);
      }

      clear(tabsContainer);
      tabs.forEach((tab) => {
        tab.classList.add("button-tab");
        tabsContainer.appendChild(tab);
      });

      showTab(tabContents.routes);
      routesTab.classList.add("active");
    })
    .catch((err) => {
      projectTitle.textContent = "Fehler beim Laden des Projekts: " + err;
    });
}
