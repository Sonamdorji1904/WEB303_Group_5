FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install

FROM node:20-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ARG NEXT_PUBLIC_PATIENT_URL=http://localhost:8081
ARG NEXT_PUBLIC_DOCTOR_URL=http://localhost:8082
ARG NEXT_PUBLIC_APPOINTMENT_URL=http://localhost:8083
ARG NEXT_PUBLIC_NOTIFICATION_URL=http://localhost:8084
ENV NEXT_PUBLIC_PATIENT_URL=$NEXT_PUBLIC_PATIENT_URL
ENV NEXT_PUBLIC_DOCTOR_URL=$NEXT_PUBLIC_DOCTOR_URL
ENV NEXT_PUBLIC_APPOINTMENT_URL=$NEXT_PUBLIC_APPOINTMENT_URL
ENV NEXT_PUBLIC_NOTIFICATION_URL=$NEXT_PUBLIC_NOTIFICATION_URL
RUN npm run build

FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
RUN addgroup --system --gid 1001 nodejs && adduser --system --uid 1001 nextjs
COPY --from=build /app/public ./public
COPY --from=build --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=build --chown=nextjs:nodejs /app/.next/static ./.next/static
USER nextjs
EXPOSE 3000
CMD ["node", "server.js"]
