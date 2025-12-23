import apiClient from "./api.js";

export const projectService = {
  /**
   * Ruft alle Projekte des Admins ab.
   * (GET /api/projects)
   */
  async getProjects() {
    try {
      const response = await apiClient.get("/projects");
      return response.data;
    } catch (err) {
      console.error("Fehler beim Abrufen der Projekte:", err);
      throw err;
    }
  },

  /**
   * Erstellt ein neues Projekt.
   * (POST /api/projects)
   * @param {string} name
   */
  async createProject(name) {
    try {
      const response = await apiClient.post("/projects", { name });
      return response.data;
    } catch (err) {
      console.error("Fehler beim Erstellen des Projekts:", err);
      throw err;
    }
  },

  /**
   * Ruft die Details für ein einzelnes Projekt ab.
   * (GET /api/projects/:id)
   * @param {string} projectID
   */
  async getProjectDetails(projectID) {
    try {
      const response = await apiClient.get(`/projects/${projectID}`);
      return response.data;
    } catch (err) {
      console.error("Fehler beim Abrufen der Projektdetails:", err);
      throw err;
    }
  },

  /**
   * Aktualisiert die Einstellungen für ein Projekt.
   * (PUT /api/projects/:id/settings)
   * @param {string} projectID
   * @param {object} settingsData
   */
  async updateProjectSettings(projectID, settingsData) {
    try {
      const response = await apiClient.put(
        `/projects/${projectID}/settings`,
        settingsData
      );
      return response.data;
    } catch (err) {
      console.error("Fehler beim Aktualisieren der Einstellungen:", err);
      throw err;
    }
  },
};
