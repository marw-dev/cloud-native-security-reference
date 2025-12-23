let _currentProjectID = null;

export const projectStore = {
  /**
   * Setzt die aktuell ausgewählte Projekt-ID.
   * @param {string | null} projectID
   */
  setCurrentProjectID(projectID) {
    _currentProjectID = projectID;
  },

  /**
   * Gibt die aktuelle Projekt-ID zurück.
   */
  getCurrentProjectID() {
    return _currentProjectID;
  },
};
