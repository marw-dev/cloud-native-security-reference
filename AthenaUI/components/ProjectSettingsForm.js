import { el } from "../lib/dom.js";
import { projectService } from "../services/project.service.js";

/**
 * Rendert das Formular für die Projekteinstellungen.
 * @param {object} props
 * @param {string} props.projectID
 * @param {object} props.project
 * @param {function} props.onSettingsUpdated
 */
export function renderProjectSettingsForm(props) {
  const { projectID, project, onSettingsUpdated } = props;

  const errorMsg = el("p", { className: "error" });
  const successMsg = el("p", { className: "success" });

  const nameInput = el("input", {
    id: "project-name",
    type: "text",
    value: project.name,
    required: true,
  });
  const hostInput = el("input", {
    id: "project-host",
    type: "text",
    // Lese den Wert aus dem sql.NullString-Objekt
    value: project.host.Valid ? project.host.String : "",
    placeholder: "z.B. webshop.meine-firma.de",
  });
  const force2FAInput = el("input", {
    type: "checkbox",
    id: "project-force-2fa",
  });
  force2FAInput.checked = project.force_2fa;

  return el(
    "form",
    {
      onSubmit: async (e) => {
        e.preventDefault();
        errorMsg.textContent = "";
        successMsg.textContent = "";

        const settingsData = {
          name: nameInput.value,
          force_2fa: force2FAInput.checked,
          host: hostInput.value,
        };

        try {
          await projectService.updateProjectSettings(projectID, settingsData);
          successMsg.textContent = "Einstellungen erfolgreich gespeichert!";

          if (onSettingsUpdated) {
            onSettingsUpdated(settingsData.name);
          }
        } catch (err) {
          errorMsg.textContent =
            err.response?.data?.error || "Fehler beim Speichern.";
        }
      },
    },
    [
      el("h4", { text: "Projekteinstellungen" }),
      el("div", {}, [el("label", { text: "Projektname:" }), nameInput]),
      el("div", {}, [
        el("label", { text: "Anwendungs-Host (Domain):" }),
        hostInput,
      ]),
      el("div", { className: "checkbox-group" }, [
        force2FAInput,
        el("label", {
          text: "Zwei-Faktor-Authentifizierung (2FA) für alle Benutzer erzwingen",
        }),
      ]),
      el("button", { type: "submit", text: "Einstellungen speichern" }),
      errorMsg,
      successMsg,
    ]
  );
}
