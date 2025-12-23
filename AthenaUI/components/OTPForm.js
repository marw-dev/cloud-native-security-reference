import { el } from "../lib/dom.js";

/**
 * Rendert ein einfaches 2FA-Eingabeformular.
 * @param {object} props
 * @param {function} props.onSubmit
 * @param {function} props.onCancel
 * @param {string} props.error
 */
export function renderOTPForm(props) {
  const { onSubmit, onCancel, error } = props;

  const otpInput = el("input", {
    type: "text",
    id: "otp",
    placeholder: "6-stelliger Code",
    required: true,
    maxLength: 6,
  });

  return el("form", { onSubmit }, [
    el("h2", { text: "Bestätigung erforderlich (2FA)" }),
    el("p", { text: "Geben Sie den Code aus Ihrer Authenticator-App ein." }),
    el("div", {}, [el("label", { text: "2FA-Code:" }), otpInput]),
    el("p", { className: "error", text: error || "" }),
    el("div", { className: "button-group" }, [
      el("button", { type: "submit", text: "Bestätigen" }),
      el("button", {
        type: "button",
        text: "Abbrechen",
        onClick: onCancel,
        className: "button-secondary",
      }),
    ]),
  ]);
}
