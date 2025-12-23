import { el, clear } from "../lib/dom.js";
import { userService } from "../services/user.service.js";

/**
 * Rendert die Benutzerverwaltungs-Komponente.
 * @param {string} projectID
 */
export function renderUserManagement(projectID) {
  const listContainer = el("div", {}, [el("p", { text: "Lade Benutzer..." })]);
  // NEU: Ein Platz für Fehler, die beim Bearbeiten/Löschen auftreten
  const globalErrorMsg = el("p", { className: "error" });

  // --- NEUE HANDLER ---

  /**
   * Handler zum Bearbeiten von Rollen
   * @param {object} user - Das vollständige Benutzerobjekt (mit ID, E-Mail, Rollen)
   */
  async function handleEditRoles(user) {
    globalErrorMsg.textContent = "";
    const newRolesString = prompt(
      `Rollen für ${user.email} bearbeiten (Komma-getrennt):`,
      user.roles.join(", ") // Zeigt die aktuellen Rollen im Prompt an
    );

    if (newRolesString === null) {
      // Benutzer hat auf "Abbrechen" geklickt
      return;
    }

    const newRoles = newRolesString
      .split(",")
      .map((r) => r.trim())
      .filter((r) => r !== "");

    if (newRoles.length === 0) {
      alert(
        "Fehler: Es muss mindestens eine Rolle (z.B. 'user') angegeben werden."
      );
      return;
    }

    try {
      await userService.updateUserRoles(projectID, user.id, newRoles);
      fetchAndRenderUsers(); // Liste neu laden
    } catch (err) {
      globalErrorMsg.textContent =
        "Fehler beim Aktualisieren der Rollen: " +
        (err.response?.data?.error || err.message);
    }
  }

  /**
   * Handler zum Entfernen eines Benutzers
   * @param {object} user - Das vollständige Benutzerobjekt
   */
  async function handleRemoveUser(user) {
    globalErrorMsg.textContent = "";
    if (
      !confirm(
        `Soll der Benutzer ${user.email} wirklich aus diesem Projekt entfernt werden?`
      )
    ) {
      return;
    }

    try {
      await userService.removeUserFromProject(projectID, user.id);
      fetchAndRenderUsers(); // Liste neu laden
    } catch (err) {
      globalErrorMsg.textContent =
        "Fehler beim Entfernen des Benutzers: " +
        (err.response?.data?.error || err.message);
    }
  }

  // --- Funktion zum Neuladen der Liste (AKTUALISIERT) ---
  async function fetchAndRenderUsers() {
    try {
      // users ist jetzt ProjectUserResponse[] (enthält .roles)
      const users = await userService.getProjectUsers(projectID);
      clear(listContainer);
      globalErrorMsg.textContent = ""; // Fehler zurücksetzen

      if (users.length === 0) {
        listContainer.appendChild(
          el("p", {
            text: "Noch keine Benutzer zu diesem Projekt hinzugefügt.",
          })
        );
      }

      // Erstelle eine Liste (JETZT MIT NEUEN DATEN)
      const userElements = users.map((user) =>
        el("div", { className: "user-karte" }, [
          el("strong", { text: user.email }),
          el("p", { text: `Globaler Admin: ${user.is_admin ? "Ja" : "Nein"}` }),
          // NEU: Zeigt die Projekt-Rollen an
          el("p", { text: `Projekt-Rollen: ${user.roles.join(", ")}` }),

          // NEU: Buttons sind jetzt funktionsfähig
          el("button", {
            text: "Rollen bearbeiten",
            onClick: () => handleEditRoles(user), // Übergibt das ganze user-Objekt
          }),
          el("button", {
            text: "Entfernen",
            onClick: () => handleRemoveUser(user),
          }),
        ])
      );
      listContainer.append(...userElements);
    } catch (err) {
      listContainer.textContent = "Fehler beim Laden der Benutzer.";
    }
  }

  // --- Formular zum Hinzufügen (unverändert) ---
  const errorMsg = el("p", { className: "error" });
  const emailInput = el("input", {
    type: "email",
    placeholder: "user@example.com",
    required: true,
  });
  const rolesInput = el("input", {
    type: "text",
    placeholder: "user,admin (Komma-getrennt)",
    value: "user",
    required: true,
  });

  const addUserForm = el(
    "form",
    {
      onSubmit: async (e) => {
        e.preventDefault();
        errorMsg.textContent = "";

        const email = emailInput.value;
        const roles = rolesInput.value
          .split(",")
          .map((r) => r.trim())
          .filter((r) => r !== ""); // Trimmen hinzugefügt

        if (roles.length === 0) {
          errorMsg.textContent =
            "Es muss mindestens eine Rolle (z.B. 'user') angegeben werden.";
          return;
        }

        try {
          await userService.addUserToProject(projectID, email, roles);
          emailInput.value = "";
          rolesInput.value = "user"; // Zurücksetzen
          fetchAndRenderUsers(); // Liste neu laden
        } catch (err) {
          errorMsg.textContent =
            err.response?.data?.error || "Fehler beim Hinzufügen.";
        }
      },
    },
    [
      el("h4", { text: "Neuen Benutzer zum Projekt einladen" }),
      el("p", {
        text: "Hinweis: Der Benutzer muss sich zuerst global registriert haben.",
      }),
      el("div", {}, [el("label", { text: "E-Mail:" }), emailInput]),
      el("div", {}, [
        el("label", { text: "Rollen (Komma-getrennt):" }),
        rolesInput,
      ]),
      el("button", { type: "submit", text: "Benutzer hinzufügen" }),
      errorMsg,
    ]
  );

  // --- Init ---
  fetchAndRenderUsers();

  // Gebe das Gesamt-Layout zurück
  return el("div", {}, [
    addUserForm,
    el("hr"),
    el("h4", { text: "Aktuelle Projekt-Benutzer" }),
    globalErrorMsg,
    listContainer,
  ]);
}
