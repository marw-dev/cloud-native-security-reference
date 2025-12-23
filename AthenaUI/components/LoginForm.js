import { el } from "../lib/dom.js";
import { navigate } from "../router/index.js";

/**
 * Rendert den reinen Formular-Baustein.
 * @param {object} props
 * @param {function} props.onSubmit
 * @param {string} props.error
 */
export function renderLoginForm(props) {
  const { onSubmit, error } = props;

  return el("form", { onSubmit }, [
    el("h2", { text: "Login" }),
    el("div", {}, [
      el("label", { text: "E-Mail:" }),
      el("input", {
        type: "email",
        id: "email",
        value: "",
        required: true,
      }),
    ]),
    el("div", {}, [
      el("label", { text: "Passwort:" }),
      el("input", {
        type: "password",
        id: "password",
        value: "",
        required: true,
      }),
    ]),

    el("button", { type: "submit", text: "Einloggen" }),
    el("p", { className: "error", text: error || "" }),
    el("p", { style: "margin-top: 15px;" }, [
      el("a", {
        href: "#/register",
        text: "Noch keinen Account? Hier registrieren.",
        onClick: (e) => {
          e.preventDefault(); // Verhindert das Neuladen der Seite
          navigate("#/register"); // Ruft den Router auf
        },
      }),
    ]),
  ]);
}
