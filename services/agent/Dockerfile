# Stage 1: build TypeScript
FROM node:20-alpine AS build
WORKDIR /build
COPY package.json package-lock.json* ./
RUN npm install --no-audit --no-fund
COPY . .
RUN npx tsc --outDir dist

# Stage 2: runtime (pre-compiled, no tsx/typescript in runtime)
FROM node:20-alpine AS runtime
WORKDIR /app
COPY --from=build /build/package.json ./
COPY --from=build /build/node_modules ./node_modules
COPY --from=build /build/dist ./dist
USER node
EXPOSE 3001
CMD ["node", "dist/server.js"]
