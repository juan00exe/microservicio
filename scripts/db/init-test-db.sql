-- Base de datos separada para los tests (se vacía en cada ejecución).
-- Postgres solo ejecuta este script al crear el volumen por primera vez.
CREATE DATABASE productos_test OWNER app;
