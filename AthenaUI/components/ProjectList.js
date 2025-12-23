import { el, clear } from "../lib/dom.js";
import { projectService } from "../services/project.service.js";
import { navigate } from "../router/index.js";

/**
 * Rendert den Baustein, der die Projektliste anzeigt.
 * @returns {HTMLElement}
 */
export function renderProjectList() {
  const projektListeContainer = el("div", { id: "projekt-liste" }, [
    el("p", { text: "Lade Projekte..." }),
  ]);

  /**
   * Interne Funktion zum Abrufen und Rendern der Daten.
   */
  async function fetchAndRenderProjects() {
    try {
      const projects = await projectService.getProjects();
      clear(projektListeContainer);

      if (projects.length === 0) {
        projektListeContainer.appendChild(
          el("p", {
            text: "Du hast noch keine Projekte. Erstelle dein erstes!",
          })
        );
        return;
      }

      const projectElements = projects.map((project) =>
        el("div", { className: "projekt-karte" }, [
          el("strong", { text: project.name }),
          el("p", { text: `ID: ${project.id}` }),
          el("button", {
            text: "Verwalten",
            onClick: () => {
              navigate(`#/project/${project.id}`);
            },
          }),
        ])
      );
      projektListeContainer.append(...projectElements);
    } catch (err) {
      projektListeContainer.textContent = "Fehler beim Laden der Projekte.";
    }
  }

  fetchAndRenderProjects();

  return {
    element: projektListeContainer,
    reload: fetchAndRenderProjects,
  };
}
