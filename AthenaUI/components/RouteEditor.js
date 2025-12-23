import { el } from "../lib/dom.js";
// NEUER IMPORT:
import { routeService } from "../services/route.service.js";

/**
 * Rendert das Formular zum Erstellen/Bearbeiten einer Route.
 * @param {object} props
 * @param {string} props.projectID
 * @param {object | null} props.routeToEdit - Die Route, die bearbeitet wird (oder null)
 * @param {function} props.onRouteSaved - Callback nach Speichern (Erstellen/Update)
 * @param {function} props.onCancel - Callback zum Abbrechen/Schließen
 */
export function renderRouteEditor(props) {
  const { projectID, routeToEdit, onRouteSaved, onCancel } = props;

  // Bestimme, ob wir im "Bearbeiten"-Modus sind
  const isEditMode = !!routeToEdit;

  const errorMsg = el("p", { className: "error" });

  // --- Basis-Felder ---
  const pathInput = el("input", {
    id: "route-path",
    type: "text",
    placeholder: "/api/meine-route/*",
    value: routeToEdit?.path || "",
    required: true,
  });
  const targetInput = el("input", {
    id: "route-target",
    type: "text",
    placeholder: "http://mein-service:8080",
    value: routeToEdit?.target_url || "",
    required: true,
  });

  // --- Erweiterte Felder ---
  const rolesInput = el("input", {
    id: "route-roles",
    type: "text",
    placeholder: "user,admin (Komma-getrennt)",
    value: routeToEdit?.required_roles?.join(", ") || "",
  });
  const cacheInput = el("input", {
    id: "route-cache",
    type: "text",
    placeholder: "z.B. 30s, 1m (0s für aus)",
    value: routeToEdit?.cache_ttl || "",
  });
  const rateLimitCountInput = el("input", {
    type: "number",
    id: "route-rl-count",
    placeholder: "z.B. 100",
    value: routeToEdit?.rate_limit?.limit || "",
  });
  const rateLimitWindowInput = el("input", {
    id: "route-rl-window",
    type: "text",
    placeholder: "z.B. 1m, 1h",
    value: routeToEdit?.rate_limit?.window || "",
  });
  const cbThresholdInput = el("input", {
    type: "number",
    id: "route-cb-thresh",
    placeholder: "z.B. 5",
    value: routeToEdit?.circuit_breaker?.failure_threshold || "",
  });
  const cbTimeoutInput = el("input", {
    id: "route-cb-timeout",
    type: "text",
    placeholder: "z.B. 30s",
    value: routeToEdit?.circuit_breaker?.open_timeout || "",
  });

  // --- Helfer zum Leeren des Formulars ---
  const clearForm = () => {
    pathInput.value = "";
    targetInput.value = "";
    rolesInput.value = "";
    cacheInput.value = "";
    rateLimitCountInput.value = "";
    rateLimitWindowInput.value = "";
    cbThresholdInput.value = "";
    cbTimeoutInput.value = "";
  };

  // --- Buttons ---
  const submitButton = el("button", {
    type: "submit",
    text: isEditMode ? "Änderungen speichern" : "Route erstellen",
  });

  const cancelButton = el("button", {
    type: "button",
    text: "Abbrechen",
    className: "button-secondary",
    onClick: (e) => {
      e.preventDefault();
      onCancel();
    },
  });

  return el(
    "form",
    {
      className: "route-editor-form",
      onSubmit: async (e) => {
        e.preventDefault();
        errorMsg.textContent = "";

        // --- Alle Daten sammeln ---
        const routeData = {
          path: pathInput.value,
          target_url: targetInput.value,
          required_roles: rolesInput.value
            .split(",")
            .map((r) => r.trim())
            .filter((role) => role !== ""),
          cache_ttl: cacheInput.value || "0s",
          rate_limit: {
            limit: parseInt(rateLimitCountInput.value) || 0,
            window: rateLimitWindowInput.value || "0s",
          },
          circuit_breaker: {
            failure_threshold: parseInt(cbThresholdInput.value) || 0,
            open_timeout: cbTimeoutInput.value || "0s",
          },
        };

        try {
          submitButton.textContent = "Speichere...";
          submitButton.disabled = true;

          if (isEditMode) {
            // --- UPDATE-LOGIK ---
            await routeService.updateProjectRoute(
              projectID,
              routeToEdit.id,
              routeData
            );
          } else {
            // --- CREATE-LOGIK ---
            await routeService.createProjectRoute(projectID, routeData);
          }

          if (!isEditMode) {
            clearForm();
          }

          if (onRouteSaved) onRouteSaved();
        } catch (err) {
          errorMsg.textContent =
            err.response?.data?.error || "Fehler beim Speichern der Route.";
        } finally {
          submitButton.textContent = isEditMode
            ? "Änderungen speichern"
            : "Route erstellen";
          submitButton.disabled = false;
        }
      },
    },
    [
      el("h4", {
        text: isEditMode
          ? `Route bearbeiten: ${routeToEdit.path}`
          : "Neue Route hinzufügen",
      }),

      el("h5", { text: "Basis-Routing" }),
      el("div", {}, [
        el("label", { text: "Pfad (z.B. /api/data/*):" }),
        pathInput,
      ]),
      el("div", {}, [
        el("label", { text: "Ziel-URL (z.B. http://service-a:8081):" }),
        targetInput,
      ]),
      el("h5", { text: "Sicherheit (Optional)" }),
      el("div", {}, [
        el("label", { text: "Benötigte Rollen (Komma-getrennt):" }),
        rolesInput,
      ]),
      el("h5", { text: "Caching (Optional)" }),
      el("div", {}, [
        el("label", { text: "Cache-Dauer (z.B. 30s, 5m):" }),
        cacheInput,
      ]),
      el("h5", { text: "Rate Limiting (Optional)" }),
      el("div", {}, [
        el("label", { text: "Anfragen (Limit):" }),
        rateLimitCountInput,
      ]),
      el("div", {}, [
        el("label", { text: "Zeitfenster (Window):" }),
        rateLimitWindowInput,
      ]),
      el("h5", { text: "Circuit Breaker (Optional)" }),
      el("div", {}, [
        el("label", { text: "Fehlerschwelle (Threshold):" }),
        cbThresholdInput,
      ]),
      el("div", {}, [
        el("label", { text: "Sperrzeit (Timeout):" }),
        cbTimeoutInput,
      ]),

      // --- Button-Container ---
      el("div", { className: "button-group" }, [
        submitButton,
        ...(isEditMode ? [cancelButton] : []),
      ]),
      errorMsg,
    ]
  );
}
