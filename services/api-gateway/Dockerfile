# Build from the repo root so the shared proto/ directory can be copied in:
#   docker build -f services/api-gateway/Dockerfile -t api-gateway .

FROM node:20-alpine AS builder

WORKDIR /app

COPY services/api-gateway/package*.json ./
RUN npm ci

COPY services/api-gateway/tsconfig.json ./
COPY services/api-gateway/src ./src
COPY services/api-gateway/graphql ./graphql
COPY proto ./proto

RUN npm run build

FROM node:20-alpine AS runtime

WORKDIR /app

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY services/api-gateway/package*.json ./
RUN npm ci --omit=dev && npm cache clean --force

COPY --from=builder /app/dist ./dist
COPY --from=builder /app/graphql ./graphql
COPY --from=builder /app/proto ./proto

# The compiled code sits in dist/, so the repo-relative proto path no longer
# applies.
ENV PROTO_DIR=/app/proto

RUN chown -R appuser:appgroup /app
USER appuser

EXPOSE 3000

CMD ["node", "dist/index.js"]
