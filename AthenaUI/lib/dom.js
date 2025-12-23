/**
 * Erstellt ein HTML-Element sicher.
 * @param {string} tag - Der HTML-Tag (z.B. 'div', 'button')
 * @param {object} [options] - Attribute wie 'id', 'className' oder 'text'
 * @param {HTMLElement[]} [children] - Untergeordnete Elemente
 * @returns {HTMLElement}
 */
export function el(tag, options = {}, children = []) {
  const element = document.createElement(tag);

  // Attribute setzen
  if (options.id) element.id = options.id;
  if (options.className) element.className = options.className;
  if (options.type) element.type = options.type;
  if (options.placeholder) element.placeholder = options.placeholder;
  if (options.value) element.value = options.value;
  if (options.required) element.required = options.required;

  if (options.text) {
    element.textContent = options.text;
  }

  if (options.onClick) {
    element.addEventListener("click", options.onClick);
  }
  if (options.onSubmit) {
    element.addEventListener("submit", options.onSubmit);
  }

  for (const child of children) {
    element.appendChild(child);
  }

  return element;
}

/**
 * Leert ein Container-Element sicher.
 * @param {HTMLElement} element
 */
export function clear(element) {
  while (element.firstChild) {
    element.removeChild(element.firstChild);
  }
}
