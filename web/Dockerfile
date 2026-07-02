# Build stage
FROM node:26-alpine AS build
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

# Runtime stage — adapter-node output is a plain Node server
FROM node:26-alpine
ENV NODE_ENV=production
USER node
WORKDIR /app
COPY --from=build /src/build ./build
COPY --from=build /src/package.json ./
EXPOSE 3000
CMD ["node", "build"]
