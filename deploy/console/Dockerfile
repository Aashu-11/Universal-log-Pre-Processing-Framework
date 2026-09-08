# The browser runs on the host, not inside the Docker network — so the
# built bundle must call the control-plane/onboarding APIs via their
# host-mapped ports (http://localhost:8000 / :8001), which is already
# src/lib/api.ts's default. No build-time API URL override needed here.
FROM node:20-alpine AS build
WORKDIR /app
COPY services/console/package.json services/console/package-lock.json ./
RUN npm ci
COPY services/console/ .
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html
# SPA fallback: any unmatched path serves index.html so React Router's
# client-side routes (e.g. /pipeline, /traceability) work on a hard refresh.
RUN printf 'server {\n\
    listen 5173;\n\
    root /usr/share/nginx/html;\n\
    location / {\n\
        try_files $uri /index.html;\n\
    }\n\
}\n' > /etc/nginx/conf.d/default.conf
EXPOSE 5173
