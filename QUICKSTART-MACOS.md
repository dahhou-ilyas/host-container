# ⚡ Démarrage Rapide sur macOS

Guide ultra-rapide pour lancer Docker Wrapper sur macOS.

## 🎯 En 5 minutes

### 1️⃣ Vérifier Docker Desktop

```bash
# Docker Desktop doit être lancé
# Icône 🐳 visible dans la barre de menu en haut

# Vérifier que Docker fonctionne
docker ps
```

**Si erreur** : Lancez Docker Desktop depuis `/Applications/Docker.app`

---

### 2️⃣ Configurer les variables d'environnement

```bash
# Le fichier .env existe déjà, éditez-le si besoin
nano .env

# Minimum requis : changez le JWT_SECRET_KEY
# Générer une clé :
openssl rand -base64 32
```

---

### 3️⃣ Lancer l'application

```bash
# Depuis le dossier du projet
cd /Users/ilyasdahhou/Downloads/docker-wrapper

# Lancer avec Docker Compose
docker-compose up -d

# Voir les logs
docker-compose logs -f api
```

---

### 4️⃣ Tester

```bash
# Test rapide
curl http://localhost:8000/health

# Devrait afficher : {"status":"ok"} ou similaire
```

---

## 🔧 Configuration macOS spécifique

### Permissions Docker sur macOS

Sur macOS avec Docker Desktop, **les permissions sont automatiques** !

Pas besoin de :
- ❌ Ajouter l'utilisateur au groupe docker (n'existe pas sur macOS)
- ❌ Modifier les permissions du socket
- ❌ Utiliser sudo

Docker Desktop gère tout automatiquement ✅

---

### Socket Docker sur macOS

Le socket est à : `/var/run/docker.sock`

Vérification :
```bash
ls -la /var/run/docker.sock
# Devrait montrer le socket
```

---

### Dossier des projets

```bash
# Créer le dossier (si nécessaire)
mkdir -p /tmp/projects

# Sur macOS, /tmp est nettoyé au redémarrage
# Pour production, utilisez un emplacement permanent :
mkdir -p ~/docker-wrapper-projects

# Puis modifiez docker-compose.yml :
# volumes:
#   - ~/docker-wrapper-projects:/tmp/projects
```

---

## ⚠️ Problèmes courants sur macOS

### "Cannot connect to Docker daemon"

**Solution :** Docker Desktop n'est pas lancé

```bash
# Lancer Docker Desktop
open -a Docker

# Attendre que l'icône 🐳 apparaisse dans la barre de menu
# Puis réessayer
```

---

### "No space left on device"

**Solution :** Nettoyer Docker

```bash
# Nettoyer les images inutilisées
docker system prune -a

# Augmenter l'espace disque alloué à Docker Desktop :
# Docker Desktop → Settings → Resources → Disk image size
```

---

### Problème de performance

**Solution :** Augmenter les ressources Docker Desktop

```bash
# Docker Desktop → Settings → Resources
# - CPU : Au moins 2 cores
# - Memory : Au moins 4 GB
# - Swap : Au moins 1 GB
```

---

## 🚀 Commandes utiles

```bash
# Démarrer
docker-compose up -d

# Arrêter
docker-compose down

# Redémarrer après modification du code
docker-compose up -d --build

# Voir les logs
docker-compose logs -f

# Voir les conteneurs créés par l'app
docker ps -a

# Entrer dans le conteneur de l'app
docker exec -it go-api sh

# Nettoyer tout
docker-compose down -v
docker system prune -a
```

---

## ✅ Checklist macOS

- [x] Docker Desktop installé et lancé (icône 🐳 visible)
- [ ] Fichier `.env` configuré
- [ ] `docker-compose up -d` sans erreur
- [ ] `curl http://localhost:8000/health` fonctionne
- [ ] Les conteneurs apparaissent dans `docker ps`

---

## 🎉 C'est prêt !

Votre application tourne maintenant et peut créer des conteneurs Docker !

Test complet :
```bash
# Voir les logs en direct
docker-compose logs -f api

# Dans un autre terminal, créez un conteneur de test
# (après avoir créé un utilisateur et obtenu un token)
```

Pour le déploiement sur AWS, consultez le guide principal dans SETUP.md

Bon développement ! 🚀
