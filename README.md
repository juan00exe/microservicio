# Microservicio de productos (Go)

CRUD REST de productos con la librería estándar (`net/http`), persistencia en PostgreSQL
(driver `pgx`) y autenticación JWT con roles. Si `DATABASE_URL` no está definida, usa
almacenamiento en memoria.

## Configuración

| Variable         | Por defecto | Descripción                                                        |
|------------------|-------------|--------------------------------------------------------------------|
| `PORT`           | `8080`      | Puerto HTTP                                                        |
| `DATABASE_URL`   | (vacía)     | Conexión a PostgreSQL; vacía = en memoria                          |
| `JWT_SECRET`     | (aleatorio) | Secreto para firmar los tokens (mín. 32 bytes). Si falta, se genera uno al arrancar y los tokens dejan de valer al reiniciar |
| `JWT_TTL`        | `1h`        | Duración de los tokens (`15m`, `1h`, `24h`…)                       |
| `ADMIN_EMAIL`    | (vacía)     | Junto con `ADMIN_PASSWORD`, crea la cuenta admin al arrancar si no existe |
| `ADMIN_PASSWORD` | (vacía)     | Contraseña de esa cuenta admin                                     |
| `LOGIN_MAX_ATTEMPTS` | `5`     | Logins fallidos por email antes de bloquear esa cuenta (0 = sin límite) |
| `LOGIN_MAX_ATTEMPTS_PER_IP` | `20` | Logins fallidos por IP, con cualquier email, antes de bloquear esa IP (0 = sin límite) |
| `LOGIN_LOCKOUT`  | `15m`       | Ventana en la que se cuentan los fallos y duración del bloqueo     |

docker-compose trae valores de desarrollo (admin `admin@example.com` / `admin12345`). **Cámbialos
en cualquier entorno real** y usa un `JWT_SECRET` largo y aleatorio, por ejemplo `openssl rand -base64 48`.

Las migraciones (`internal/database/migrations/*.sql`) se aplican automáticamente al arrancar.

## Ejecutar

Todo con Docker (API + PostgreSQL):

```bash
docker compose up -d --build
```

Solo la base de datos en Docker y la API en local:

```bash
docker compose up -d db
DATABASE_URL="postgres://app:app@localhost:5432/productos?sslmode=disable" go run ./cmd/server
```

## Tests

```bash
go test ./...   # solo memoria
TEST_DATABASE_URL="postgres://app:app@localhost:5432/productos_test?sslmode=disable" go test ./...   # también PostgreSQL
```

Los tests de PostgreSQL **vacían la tabla `products`**, así que usan su propia base de datos
`productos_test` (nunca apuntes `TEST_DATABASE_URL` a `productos`). docker-compose la crea al
inicializar un volumen nuevo; si tu volumen es anterior, créala una vez con:

```bash
docker compose exec db createdb -U app productos_test
```

## Documentación interactiva (Swagger UI)

Con el servicio en marcha, abre **http://localhost:8080/docs** para ver y probar la API desde el
navegador ("Try it out" → "Execute"). La especificación OpenAPI está en `/openapi.yaml`
(fuente: `internal/docs/openapi.yaml`; actualízala al cambiar los endpoints).
Swagger UI se carga desde el CDN de jsDelivr, así que el navegador necesita internet.

## Autenticación

| Operación                              | Quién puede                 |
|----------------------------------------|-----------------------------|
| `GET /products`, `GET /products/{id}`  | Cualquiera (sin token)      |
| `POST`, `PUT`, `DELETE /products…`     | Solo rol **admin**          |
| `GET /auth/me`                         | Cualquier usuario con token |

- `POST /auth/register` `{"email","password"}` crea una cuenta con rol **user** (contraseña de 8 a 72 caracteres, guardada con bcrypt).
- `POST /auth/login` devuelve `{"access_token","token_type":"Bearer","expires_in"}`.
- Envía el token en la cabecera `Authorization: Bearer <access_token>`.
- Sin token o con token inválido/expirado → **401**; con token de user en una ruta de admin → **403**.
- El rol va dentro del token: si cambia el rol de alguien, se aplica cuando vuelva a hacer login.
- **Bloqueo por intentos fallidos:** 5 logins fallidos con un email (o 20 desde una IP) en 15 minutos
  bloquean ese email (o esa IP) durante 15 minutos → **429** con cabecera `Retry-After`, aunque la
  contraseña sea correcta. Un login correcto reinicia el contador del email.
  - Los contadores viven en memoria: se reinician al reiniciar el servicio y, con varias
    instancias, cada una lleva su cuenta.
  - La IP es la de la conexión. Detrás de un proxy o balanceador todos los clientes compartirían
    la IP del proxy, así que habría que adaptar `clientIP` para leer la cabecera del proxy.

```bash
TOKEN=$(curl -s localhost:8080/auth/login -d '{"email":"admin@example.com","password":"admin12345"}' | jq -r .access_token)
```

En Swagger UI (`/docs`): haz login en `POST /auth/login`, copia `access_token` y pégalo en **Authorize**.

## Postman

La colección `postman/microservicio.postman_collection.json` prueba todo el flujo (health, registro
y login, permisos 401/403, CRUD, paginación, filtro y errores) con verificaciones automáticas.
Usa el admin de las variables `adminEmail`/`adminPassword` de la colección (por defecto, el de docker-compose).

1. En Postman: **Import** → selecciona el archivo.
2. Con el servicio en marcha, botón derecho en la colección → **Run collection** → **Run**.
   Las peticiones se encadenan (el `id` creado se guarda en la variable `productId`), así que
   ejecútalas en orden. Para otro servidor, cambia la variable `baseUrl` de la colección.

Desde la terminal, con Newman: `npx newman run postman/microservicio.postman_collection.json`

## Endpoints

| Método | Ruta              | Descripción          |
|--------|-------------------|----------------------|
| GET    | `/health`         | Estado del servicio  |
| GET    | `/products`       | Listar productos     |
| GET    | `/products/{id}`  | Obtener un producto  |
| POST   | `/products`       | Crear producto       |
| PUT    | `/products/{id}`  | Actualizar producto  |
| DELETE | `/products/{id}`  | Borrar producto      |

Cuerpo para POST/PUT:

```json
{ "name": "Teclado", "description": "Mecánico", "price": 49.9, "stock": 10 }
```

### Paginación y filtro

`GET /products` acepta `page` (desde 1, por defecto 1), `limit` (1–100, por defecto 20) y
`name` (contiene el texto, sin distinguir mayúsculas). Valores fuera de rango devuelven 400.

```json
{
  "items": [ { "id": "…", "name": "Teclado", "...": "…" } ],
  "page": 1,
  "limit": 20,
  "total": 42,
  "total_pages": 3
}
```

## Ejemplos

```bash
# Lecturas públicas
curl localhost:8080/products
curl "localhost:8080/products?page=2&limit=10&name=teclado"

# Escrituras con token de admin ($TOKEN: ver "Autenticación")
curl -X POST localhost:8080/products -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"Teclado","price":49.9,"stock":10}'
curl -X PUT localhost:8080/products/<id> -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"Teclado","price":39.9,"stock":8}'
curl -X DELETE localhost:8080/products/<id> -H "Authorization: Bearer $TOKEN"
```

## Estructura

```
cmd/server/        punto de entrada (config, logging, graceful shutdown)
internal/auth/     usuarios, login, JWT y middleware de roles
internal/config/   configuración por variables de entorno
internal/database/ conexión a PostgreSQL, migraciones embebidas y helper de tests (dbtest)
internal/httpx/    helpers JSON y middleware (logging, recover)
internal/ids/      generación de UUID
internal/product/  modelo, repositorios (memoria y PostgreSQL), servicio y handlers HTTP
```

`GET /health` también comprueba la conexión a la base de datos y devuelve 503 si no responde.
