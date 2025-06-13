# Multi-stage Dockerfile for React/Next.js Web UI

# Stage 1: Build stage
FROM node:20-alpine AS build-stage

# Set working directory
WORKDIR /app

# Copy package files for dependency caching
COPY package.json pnpm-lock.yaml ./

# Install pnpm and dependencies
RUN npm install -g pnpm && pnpm install --frozen-lockfile

# Copy source code
COPY . .

# Build the application
RUN pnpm run build

# Stage 2: Production stage
FROM node:20-alpine AS production-stage

# Set working directory
WORKDIR /app

# Copy built Next.js standalone files
COPY --from=build-stage /app/.next/standalone ./
COPY --from=build-stage /app/.next/static ./.next/static
COPY --from=build-stage /app/public ./public

# Expose port 8080
EXPOSE 8080

# Set environment for production
ENV NODE_ENV=production
ENV PORT=8080

# Start the Next.js server
CMD ["node", "server.js"]