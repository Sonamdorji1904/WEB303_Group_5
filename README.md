# WEB303_Group_5

## Running the project

Start the full hospital system with one shared RabbitMQ, service-owned
PostgreSQL databases, four Go APIs, and the Next.js dashboard:

```sh
docker compose up --build -d
docker compose ps
```

Stop the stack without deleting its named volumes:

```sh
docker compose down
```

| Service | Host port | URL |
|---|---:|---|
| Patient API | 8081 | http://localhost:8081 |
| Doctor API | 8082 | http://localhost:8082 |
| Appointment API | 8083 | http://localhost:8083 |
| Notification API | 8084 | http://localhost:8084 |
| Frontend dashboard | 3000 | http://localhost:3000 |
| RabbitMQ AMQP | 5672 | amqp://guest:guest@localhost:5672/ |
| RabbitMQ management | 15672 | http://localhost:15672 |
| Patient PostgreSQL | 5433 | localhost:5433 |
| Notification PostgreSQL | 5435 | localhost:5435 |
| Doctor PostgreSQL | 5434 | localhost:5434 |
| Appointment PostgreSQL | 5436 | localhost:5436 |

RabbitMQ management uses the local development credentials `guest` / `guest`.

The Doctor and Appointment services consume and publish events through the same
RabbitMQ exchange. Their databases remain service-owned and isolated.

Doctor approval flow: booking an available slot creates a `PENDING` appointment,
puts the slot in `HELD`, and creates a request at
`http://localhost:8082/api/requests`. Open the **Doctor requests** page in the
dashboard to accept or reject it. Rejection requires a reason, releases the
slot, and displays that reason on the rejected appointment.

For local frontend development outside Docker:

```sh
cd frontend
cp .env.example .env.local
npm install
npm run dev
```

The browser uses the `NEXT_PUBLIC_*` URLs in `.env.local`, which must point to
host-reachable API ports. The frontend is configured for `http://localhost:3000`
and the Compose environment enables CORS for that origin.