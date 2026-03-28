# Build stage
FROM node:18-alpine AS builder

WORKDIR /app

# Copy package.json and package-lock.json (if available)
COPY package*.json ./

# Install dependencies
RUN npm install

# Copy the source code
COPY . .

# Build the application
RUN npm run build

# Production stage
FROM nginx:alpine

# Copy the build output to replace the default nginx contents
COPY --from=builder /app/dist /usr/share/nginx/html

# Overwrite the default nginx configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Expose port 80 (mapped to 3000 in docker-compose)
EXPOSE 80

# Start Nginx
CMD ["nginx", "-g", "daemon off;"]
