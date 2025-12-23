import apiClient from "./api.js";

export const userService = {
  /**
   * (GET /api/users/me)
   * WICHTIG: Benötigt eine X-Project-ID im Header,
   * um den Kontext für Rollen/OTP zu bestimmen.
   */
  async getProfile() {
    try {
      const response = await apiClient.get("/users/me");
      return response.data; // Gibt ProfileResponse zurück
    } catch (err) {
      console.error("Fehler beim Abrufen des Benutzerprofils:", err);
      throw err;
    }
  },

  /**
   * Lädt alle Benutzer für ein Projekt.
   * (GET /api/projects/:id/users)
   */
  async getProjectUsers(projectID) {
    try {
      const response = await apiClient.get(`/projects/${projectID}/users`);
      return response.data; // Gibt jetzt die ProjectUserResponse[] zurück
    } catch (err) {
      console.error("Fehler beim Abrufen der Projekt-Benutzer:", err);
      throw err;
    }
  },

  /**
   * Fügt einen Benutzer (per E-Mail) zu einem Projekt hinzu.
   * (POST /api/projects/:id/users)
   */
  async addUserToProject(projectID, email, roles) {
    try {
      const response = await apiClient.post(`/projects/${projectID}/users`, {
        email,
        roles,
      });
      return response.data;
    } catch (err) {
      console.error("Fehler beim Hinzufügen des Benutzers:", err);
      throw err;
    }
  },

  /**
   * NEU: Aktualisiert die Rollen eines Benutzers in einem Projekt.
   * (PUT /api/projects/:id/users/:userID)
   * @param {string} projectID
   * @param {string} userID
   * @param {string[]} roles
   */
  async updateUserRoles(projectID, userID, roles) {
    try {
      const response = await apiClient.put(
        `/projects/${projectID}/users/${userID}`,
        { roles } // Sendet { "roles": ["user", "admin"] }
      );
      return response.data;
    } catch (err) {
      console.error("Fehler beim Aktualisieren der Benutzerrollen:", err);
      throw err;
    }
  },

  /**
   * NEU: Entfernt einen Benutzer aus einem Projekt.
   * (DELETE /api/projects/:id/users/:userID)
   * @param {string} projectID
   * @param {string} userID
   */
  async removeUserFromProject(projectID, userID) {
    try {
      const response = await apiClient.delete(
        `/projects/${projectID}/users/${userID}`
      );
      return response.data;
    } catch (err) {
      console.error("Fehler beim Entfernen des Benutzers:", err);
      throw err;
    }
  },
};
