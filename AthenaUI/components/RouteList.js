import { el, clear } from "../lib/dom.js";
import { routeService } from "../services/route.service.js";

/**
 * Rendert die Liste der Routen für ein Projekt.
 * @param {string} projectID
 * @param {function} onEditRoute
 */
export function renderRouteList(projectID, onEditRoute) {
  // NEU: onEditRoute
  const container = el("div", {}, [
    el("h4", { text: "Konfigurierte Routen" }),
    el("p", { text: "Lade Routen..." }),
  ]);

  async function handleDelete(routeID, path) {
    if (!confirm(`Soll die Route "${path}" wirklich gelöscht werden?`)) {
      return;
    }
    try {
      await routeService.deleteProjectRoute(projectID, routeID);
      fetchAndRenderRoutes();
    } catch (err) {
      alert("Fehler beim Löschen der Route: " + err.message);
    }
  }

  async function fetchAndRenderRoutes() {
    try {
      // Service-Aufruf geändert:
      const routes = await routeService.getProjectRoutes(projectID);
      clear(container);

      if (routes.length === 0) {
        container.appendChild(
          el("p", {
            text: "Noch keine Routen für dieses Projekt konfiguriert.",
          })
        );
        return;
      }

      const tableHeader = el("tr", {}, [
        el("th", { text: "Pfad" }),
        el("th", { text: "Ziel-URL" }),
        el("th", { text: "Rollen" }),
        el("th", { text: "Aktionen" }),
      ]);

      const tableBody = routes.map((route) =>
        el("tr", {}, [
          el("td", { text: route.path }),
          el("td", { text: route.target_url }),
          el("td", { text: route.required_roles.join(", ") || "-" }),
          el("td", {}, [
            el("button", {
              text: "Bearbeiten",
              onClick: () => onEditRoute(route),
            }),
            el("button", {
              text: "Löschen",
              onClick: () => handleDelete(route.id, route.path),
            }),
          ]),
        ])
      );

      container.appendChild(
        el("table", {}, [
          el("thead", {}, [tableHeader]),
          el("tbody", {}, tableBody),
        ])
      );
    } catch (err) {
      container.textContent = "Fehler beim Laden der Routen.";
    }
  }

  fetchAndRenderRoutes();

  return {
    element: container,
    reload: fetchAndRenderRoutes,
  };
}
