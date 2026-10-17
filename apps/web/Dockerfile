FROM node:24-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ENV NEXT_TELEMETRY_DISABLED=1
ENV API_URL=http://api:8080
RUN npm run build
USER node
EXPOSE 3000
CMD ["npm","start"]
