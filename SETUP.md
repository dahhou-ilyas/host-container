# 🚀 Guide de Configuration - Docker Wrapper

Ce guide vous aide à configurer correctement Docker et les permissions pour lancer l'application.

## 📋 Prérequis

- Docker installé sur votre machine
- Docker Compose installé
- Accès sudo/administrateur (pour la configuration initiale)

---

## 🔧 Étape 1 : Configuration des variables d'environnement

### 1.1 Vérifier le fichier `.env`

Un fichier `.env` a été créé avec des valeurs par défaut. Modifiez-le selon vos besoins :

```bash
# Éditer le fichier .env
nano .env
# ou
vi .env
```

### 1.2 Variables importantes à configurer

```bash
# Base de données (à adapter selon votre configuration PostgreSQL)
DATABASE_URL=postgres://user:password@host:port/database

# Clé secrète JWT (IMPORTANT: changez-la en production !)
# Générer une clé forte :
openssl rand -base64 32

# Puis collez le résultat dans .env :
JWT_SECRET_KEY=votre-clé-générée-ici
```

---

## 🐳 Étape 2 : Vérifier les permissions Docker

### 2.1 Lancer le script de vérification

```bash
# Rendre le script exécutable
chmod +x check-docker-permissions.sh

# Lancer le script
./check-docker-permissions.sh
```

### 2.2 Corrections communes

#### Problème : "Permission denied" avec Docker

**Solution :** Ajouter votre utilisateur au groupe `docker`

```bash
# Sur Linux
sudo usermod -aG docker $USER

# Puis reconnectez-vous ou exécutez :
newgrp docker

# Sur macOS
# Docker Desktop gère automatiquement les permissions
# Assurez-vous que Docker Desktop est lancé
```

#### Problème : "Cannot connect to Docker daemon"

**Solutions :**

```bash
# Sur Linux
sudo systemctl start docker
sudo systemctl enable docker  # Démarrage automatique

# Sur macOS
# Lancez Docker Desktop depuis Applications

# Sur Windows WSL2
# Lancez Docker Desktop pour Windows
```

#### Problème : Le socket Docker n'existe pas

**Vérification :**

```bash
# Vérifier si le socket existe
ls -la /var/run/docker.sock

# Devrait afficher quelque chose comme :
# srw-rw---- 1 root docker 0 Jan 11 10:00 /var/run/docker.sock
```

---

## 📁 Étape 3 : Créer le dossier des projets

```bash
# Créer le dossier où seront stockés les projets utilisateurs
mkdir -p /tmp/projects

# Vérifier les permissions
ls -ld /tmp/projects

# Option : Utiliser un emplacement permanent (recommandé en production)
# mkdir -p /var/lib/docker-wrapper/projects
# Puis modifier PROJECTS_BASE_PATH dans .env
```

---

## 🧪 Étape 4 : Tester la configuration

### 4.1 Test basique Docker

```bash
# Vérifier que Docker fonctionne sans sudo
docker ps

# Devrait afficher la liste des conteneurs (même vide)
# Si erreur "permission denied", retournez à l'étape 2.2
```

### 4.2 Test du socket Docker

```bash
# Créer un conteneur de test qui accède au socket
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  docker:latest docker ps

# Devrait afficher la liste des conteneurs
# Si ça fonctionne, votre app fonctionnera aussi !
```

---

## 🚀 Étape 5 : Lancer l'application

### 5.1 Mode développement

```bash
# Build et lancement
docker-compose up --build

# Ou en arrière-plan
docker-compose up -d --build

# Voir les logs
docker-compose logs -f api
```

### 5.2 Vérifier que l'app fonctionne

```bash
# Test du endpoint health
curl http://localhost:8000/health

# Devrait retourner quelque chose comme :
# {"status": "ok"}
```

### 5.3 Tester la création de conteneurs

```bash
# 1. S'enregistrer (exemple)
curl -X POST http://localhost:8000/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test User",
    "email": "test@example.com",
    "password": "password123"
  }'

# 2. Se connecter
curl -X POST http://localhost:8000/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }'

# 3. Créer un conteneur (avec le token reçu)
curl -X POST http://localhost:8000/containers/create \
  -H "Authorization: Bearer VOTRE_TOKEN_ICI" \
  -H "Content-Type: application/json" \
  -d '{
    "project_name": "mon-projet",
    "image": "ubuntu:latest"
  }'

# 4. Vérifier que le conteneur a été créé
docker ps | grep ubuntu
```

---

## 🔍 Dépannage

### Logs de l'application

```bash
# Voir les logs en temps réel
docker-compose logs -f api

# Voir les dernières lignes
docker-compose logs --tail=50 api
```

### Entrer dans le conteneur

```bash
# Ouvrir un shell dans le conteneur
docker exec -it go-api sh

# Vérifier le socket depuis l'intérieur
ls -la /var/run/docker.sock

# Tester Docker depuis l'intérieur
docker ps
```

### Problèmes de connexion à la base de données

```bash
# Vérifier que PostgreSQL est accessible
psql "postgres://user:password@host:port/database"

# Ou avec Docker (si vous utilisez PostgreSQL en conteneur)
docker run --rm -it postgres:latest psql "YOUR_DATABASE_URL"
```

### Nettoyer et redémarrer

```bash
# Arrêter tous les conteneurs
docker-compose down

# Supprimer les volumes (⚠️ efface les données)
docker-compose down -v

# Nettoyer les images
docker system prune -a

# Redémarrer proprement
docker-compose up --build
```

---

## 🔒 Sécurité en Production

### 1. Ne JAMAIS committer le fichier `.env`

```bash
# Vérifier que .env est dans .gitignore
cat .gitignore | grep .env

# Devrait afficher :
# .env
# .env.local
# *.env
```

### 2. Utiliser des secrets forts

```bash
# Générer un JWT secret fort
openssl rand -base64 32

# Générer un mot de passe de base de données fort
openssl rand -base64 24
```

### 3. Limiter les permissions du socket Docker

En production, utilisez un proxy de socket Docker :

```yaml
# Ajouter dans docker-compose.yml
services:
  docker-proxy:
    image: tecnativa/docker-socket-proxy
    environment:
      CONTAINERS: 1
      POST: 1
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
```

---

## ✅ Checklist finale

Avant de déployer en production, vérifiez :

- [ ] Le fichier `.env` est configuré avec des valeurs de production
- [ ] Le `JWT_SECRET_KEY` est une clé forte et unique
- [ ] La connexion à la base de données PostgreSQL fonctionne
- [ ] Le socket Docker est accessible
- [ ] Les permissions sont correctement configurées
- [ ] Le dossier `/tmp/projects` (ou équivalent) existe
- [ ] Le fichier `.env` n'est PAS commité dans Git
- [ ] L'application démarre sans erreur
- [ ] Vous pouvez créer des conteneurs via l'API
- [ ] Les logs ne montrent pas d'erreurs

---

## 📚 Ressources

- [Documentation Docker](https://docs.docker.com/)
- [Docker Compose Reference](https://docs.docker.com/compose/compose-file/)
- [PostgreSQL Connection Strings](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING)
- [JWT Best Practices](https://tools.ietf.org/html/rfc8725)

---

## 🆘 Besoin d'aide ?

Si vous rencontrez des problèmes :

1. Vérifiez les logs : `docker-compose logs -f`
2. Lancez le script de vérification : `./check-docker-permissions.sh`
3. Consultez la section Dépannage ci-dessus
4. Vérifiez que toutes les étapes ont été suivies

Bonne chance ! 🚀
