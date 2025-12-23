import { authStore } from "../store/auth.store.js";
import { projectStore } from "../store/project.store.js";

const API_URL = "http://localhost:8080/api";

const apiClient = axios.create({
  baseURL: API_URL,
});

apiClient.interceptors.request.use((config) => {
  const authState = authStore.getState();
  const projectID = projectStore.getCurrentProjectID(); // Kann 'null' sein

  if (authState.token) {
    config.headers.Authorization = `Bearer ${authState.token}`;
  }

  // Sende die ID, WENN sie gesetzt ist (und nicht null)
  if (projectID) {
    config.headers["X-Project-ID"] = projectID;
  } else {
    // Stelle dass sie entfernt wird, wenn sie null ist (wichtig für Admin-Aufrufe)
    delete config.headers["X-Project-ID"];
  }

  return config;
});

export default apiClient;
