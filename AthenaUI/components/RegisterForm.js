import { el } from "../lib/dom.js";
import { navigate } from "../router/index.js";

/**
 * Rendert den reinen Registrierungs-Formular-Baustein.
 * @param {object} props
 * @param {function} props.onSubmit
 * @param {string} props.error
 * @param {string} props.success
 */
export function renderRegisterForm(props) {
  const { onSubmit, error, success } = props;

  return el("form", { onSubmit }, [
    el("h2", { text: "Account erstellen" }),
    el("div", {}, [
      el("label", { text: "E-Mail:" }),
      el("input", { type: "email", id: "email", required: true }),
    ]),
    el("div", {}, [
      el("label", { text: "Passwort (min. 8 Zeichen):" }),
      el("input", { type: "password", id: "password", required: true }),
    ]),
    el("div", {}, [
      el("label", { text: "Passwort bestätigen:" }),
      el("input", { type: "password", id: "confirmPassword", required: true }),
    ]),

    el("div", {}, [
      el("label", { text: "Registrierungscode:" }),
      el("input", {
        type: "password",
        id: "registrationSecret",
        required: true,
      }),
    ]),

    el("button", { type: "submit", text: "Registrieren" }),
    el("p", { className: "error", text: error || "" }),
    el("p", { className: "success", text: success || "" }),
    el("p", {}, [
      el("a", {
        href: "#/login",
        text: "Bereits einen Account? Hier einloggen.",
        onClick: (e) => {
          e.preventDefault();
          navigate("#/login");
        },
      }),
    ]),
  ]);
}
