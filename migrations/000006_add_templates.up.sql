CREATE TABLE templates (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT NOT NULL,
    description   TEXT NOT NULL,
    image         TEXT NOT NULL,
    category      TEXT NOT NULL DEFAULT 'other',
    tags          JSONB NOT NULL DEFAULT '[]',
    popular       BOOLEAN NOT NULL DEFAULT FALSE,
    icon          TEXT,
    default_ports JSONB NOT NULL DEFAULT '[]',
    created_at    TIMESTAMP DEFAULT NOW()
);

INSERT INTO templates (name, description, image, category, tags, popular, icon, default_ports) VALUES
('Nginx',      'Serveur web et reverse proxy haute performance',  'nginx:alpine',    'web',      '["web","http","proxy"]',          true,  'nginx',    '[{"internal":80,"external":8080,"protocol":"tcp"}]'),
('Node.js',    'Environnement d''exécution JavaScript',          'node:20-alpine',  'runtime',  '["javascript","node","npm"]',     true,  'nodejs',   '[{"internal":3000,"external":3000,"protocol":"tcp"}]'),
('Python',     'Environnement Python 3',                         'python:3-alpine', 'runtime',  '["python","pip"]',                true,  'python',   '[]'),
('PostgreSQL', 'Base de données relationnelle robuste',          'postgres:16',     'database', '["database","sql","postgres"]',   true,  'postgres', '[{"internal":5432,"external":5432,"protocol":"tcp"}]'),
('Redis',      'Cache et broker de messages en mémoire',         'redis:alpine',    'database', '["cache","redis","nosql"]',       true,  'redis',    '[{"internal":6379,"external":6379,"protocol":"tcp"}]'),
('MySQL',      'Base de données relationnelle populaire',        'mysql:8',         'database', '["database","sql","mysql"]',      false, 'mysql',    '[{"internal":3306,"external":3306,"protocol":"tcp"}]'),
('MongoDB',    'Base de données orientée documents',             'mongo:7',         'database', '["database","nosql","mongo"]',    false, 'mongo',    '[{"internal":27017,"external":27017,"protocol":"tcp"}]'),
('Alpine',     'Image Linux ultra-légère',                       'alpine:latest',   'tools',    '["linux","shell","minimal"]',     false, 'alpine',   '[]'),
('Ubuntu',     'Environnement Linux Ubuntu complet',             'ubuntu:22.04',    'tools',    '["linux","ubuntu","shell"]',      false, 'ubuntu',   '[]'),
('RabbitMQ',   'Message broker AMQP',                           'rabbitmq:alpine', 'tools',    '["messaging","amqp","queue"]',    false, 'rabbitmq', '[{"internal":5672,"external":5672,"protocol":"tcp"}]');
