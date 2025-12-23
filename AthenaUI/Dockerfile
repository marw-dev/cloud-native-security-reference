FROM node:24-alpine AS builder

WORKDIR /app

# Umgebungsvariable für die API-URL (kann beim Build überschrieben werden)
ARG VITE_API_URL=/api

ENV VITE_API_URL=$VITE_API_URL


COPY package.json package-lock.json ./
RUN npm install

# Kopiere den Rest des UI-Codes
COPY . .

# Baue die App für die Produktion
# VITE_API_URL wird hier in den statischen Code einkompiliert
RUN npm run build

# --- Stage 2: Finales, minimales Nginx-Image ---
FROM nginx:1.29-alpine

# Kopiere die statischen Build-Assets aus Stage 1
COPY --from=builder /app/dist /usr/share/nginx/html

# (Optional) Kopiere eine Nginx-Config, falls Sie Client-Side-Routing (History-Mode) verwenden
# COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]