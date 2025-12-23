import { el } from "../lib/dom.js";
import { projectService } from "../services/project.service.js";

/**
 * Rendert den Baustein zum Erstellen von Projekten.
 * @param {object} props
 * @param {function} props.onProjectCreated
 */
export function renderCreateProjectForm(props) {
  const { onProjectCreated } = props;

  const createFormError = el("p", { className: "error" });
  const createFormInput = el("input", {
    type: "text",
    id: "new-project-name",
    placeholder: "Name des neuen Projekts",
    required: true,
  });
  const createFormButton = el("button", {
    type: "submit",
    text: "Projekt erstellen",
  });

  return el(
    "form",
    {
      className: "create-project-form",
      onSubmit: async (e) => {
        e.preventDefault();
        createFormButton.textContent = "Erstelle...";
        createFormError.textContent = "";

        try {
          const newProjectName = createFormInput.value;
          await projectService.createProject(newProjectName);

          createFormInput.value = "";

          if (onProjectCreated) {
            onProjectCreated();
          }
        } catch (err) {
          createFormError.textContent =
            err.response?.data?.error || "Erstellung fehlgeschlagen.";
        } finally {
          createFormButton.textContent = "Projekt erstellen";
        }
      },
    },
    [
      el("h4", { text: "Neues Projekt erstellen" }),
      createFormInput,
      createFormButton,
      createFormError,
    ]
  );
}
