import apiClient from "./api.js";

export const routeService = {
  /**
   * Ruft alle Routen für ein bestimmtes Projekt ab.
   * (GET /api/projects/:id/routes)
   * @param {string} projectID
   */
  async getProjectRoutes(projectID) {
    try {
      const response = await apiClient.get(`/projects/${projectID}/routes`);
      return response.data;
    } catch (err) {
      console.error("Fehler beim Abrufen der Projektrouten:", err);
      throw err;
    }
  },

  /**
   * Erstellt eine neue Route für ein Projekt.
   * (POST /api/projects/:id/routes)
   * @param {string} projectID
   * @param {object} routeData
   */
  async createProjectRoute(projectID, routeData) {
    try {
      const response = await apiClient.post(
        `/projects/${projectID}/routes`,
        routeData
      );
      return response.data;
    } catch (err) {
      console.error("Fehler beim Erstellen der Route:", err);
      throw err;
    }
  },

  /**
   * NEU: Aktualisiert eine bestehende Route.
   * (PUT /api/projects/:id/routes/:routeID)
   * @param {string} projectID
   * @param {string} routeID
   * @param {object} routeData
   */
  async updateProjectRoute(projectID, routeID, routeData) {
    try {
      const response = await apiClient.put(
        `/projects/${projectID}/routes/${routeID}`,
        routeData
      );
      return response.data;
    } catch (err) {
      console.error("Fehler beim Aktualisieren der Route:", err);
      throw err;
    }
  },

  /**
   * NEU (aus Refactoring): Löscht eine Route aus einem Projekt.
   * (DELETE /api/projects/:id/routes/:routeID)
   * @param {string} projectID
   * @param {string} routeID
   */
  async deleteProjectRoute(projectID, routeID) {
    try {
      const response = await apiClient.delete(
        `/projects/${projectID}/routes/${routeID}`
      );
      return response.data;
    } catch (err) {
      console.error("Fehler beim Löschen der Route:", err);
      throw err;
    }
  },
};
