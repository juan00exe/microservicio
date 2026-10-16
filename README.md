# Microservicio de productos (Go)

CRUD REST de productos con la librería estándar (`net/http`) y persistencia en PostgreSQL
(driver `pgx`). Si `DATABASE_URL` no está definida, usa almacenamiento en memoria.

## Configuración

| Variable       | Por defecto | Descripción                                   |
|----------------|-------------|-----------------------------------------------|
| `PORT`         | `8080`      | Puerto HTTP                                   |
| `DATABASE_URL` | (vacía)     | Conexión a PostgreSQL; vacía = en memoria     |

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
TEST_DATABASE_URL="postgres://app:app@localhost:5432/productos?sslmode=disable" go test ./...   # también PostgreSQL
```

Los tests de PostgreSQL vacían la tabla `products`; usa una base de datos de pruebas.

## Documentación interactiva (Swagger UI)

Con el servicio en marcha, abre **http://localhost:8080/docs** para ver y probar la API desde el
navegador ("Try it out" → "Execute"). La especificación OpenAPI está en `/openapi.yaml`
(fuente: `internal/docs/openapi.yaml`; actualízala al cambiar los endpoints).
Swagger UI se carga desde el CDN de jsDelivr, así que el navegador necesita internet.

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

## Ejemplos

```bash
curl -X POST localhost:8080/products -H "Content-Type: application/json" \
  -d '{"name":"Teclado","price":49.9,"stock":10}'
curl localhost:8080/products
curl -X PUT localhost:8080/products/<id> -d '{"name":"Teclado","price":39.9,"stock":8}'
curl -X DELETE localhost:8080/products/<id>
```

## Estructura

```
cmd/server/        punto de entrada (config, logging, graceful shutdown)
internal/config/   configuración por variables de entorno
internal/database/ conexión a PostgreSQL y migraciones embebidas
internal/httpx/    helpers JSON y middleware (logging, recover)
internal/product/  modelo, repositorios (memoria y PostgreSQL), servicio y handlers HTTP
```

`GET /health` también comprueba la conexión a la base de datos y devuelve 503 si no responde.
