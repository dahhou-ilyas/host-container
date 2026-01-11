#!/bin/bash

# Script pour vérifier et configurer les permissions Docker
# Usage: ./check-docker-permissions.sh

set -e

echo "======================================"
echo "  Docker Permissions Checker"
echo "======================================"
echo ""

# Couleurs pour l'affichage
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 1. Vérifier si Docker est installé
echo "1. Vérification de l'installation Docker..."
if command -v docker &> /dev/null; then
    echo -e "${GREEN}✓${NC} Docker est installé"
    docker --version
else
    echo -e "${RED}✗${NC} Docker n'est pas installé"
    echo "   Installez Docker depuis: https://docs.docker.com/get-docker/"
    exit 1
fi
echo ""

# 2. Vérifier si le daemon Docker tourne
echo "2. Vérification du daemon Docker..."
if docker info &> /dev/null; then
    echo -e "${GREEN}✓${NC} Le daemon Docker est actif"
else
    echo -e "${RED}✗${NC} Le daemon Docker n'est pas actif"
    echo "   Démarrez Docker avec: sudo systemctl start docker"
    exit 1
fi
echo ""

# 3. Vérifier le socket Docker
echo "3. Vérification du socket Docker..."
if [ -S /var/run/docker.sock ]; then
    echo -e "${GREEN}✓${NC} Le socket Docker existe: /var/run/docker.sock"
    ls -la /var/run/docker.sock
else
    echo -e "${RED}✗${NC} Le socket Docker n'existe pas"
    exit 1
fi
echo ""

# 4. Vérifier les permissions du socket
echo "4. Vérification des permissions du socket..."
SOCKET_PERMS=$(stat -f "%Sp" /var/run/docker.sock 2>/dev/null || stat -c "%A" /var/run/docker.sock 2>/dev/null)
SOCKET_GROUP=$(stat -f "%Sg" /var/run/docker.sock 2>/dev/null || stat -c "%G" /var/run/docker.sock 2>/dev/null)
echo "   Permissions: $SOCKET_PERMS"
echo "   Groupe: $SOCKET_GROUP"
echo ""

# 5. Vérifier si l'utilisateur actuel est dans le groupe docker
echo "5. Vérification de l'appartenance au groupe docker..."
CURRENT_USER=$(whoami)
if groups $CURRENT_USER | grep -q docker; then
    echo -e "${GREEN}✓${NC} L'utilisateur '$CURRENT_USER' est dans le groupe 'docker'"
else
    echo -e "${YELLOW}⚠${NC} L'utilisateur '$CURRENT_USER' n'est PAS dans le groupe 'docker'"
    echo ""
    echo "   Pour ajouter l'utilisateur au groupe docker:"
    echo "   ${YELLOW}sudo usermod -aG docker $CURRENT_USER${NC}"
    echo ""
    echo "   Puis déconnectez-vous et reconnectez-vous, ou exécutez:"
    echo "   ${YELLOW}newgrp docker${NC}"
    echo ""
fi
echo ""

# 6. Tester l'accès Docker sans sudo
echo "6. Test de l'accès Docker sans sudo..."
if docker ps &> /dev/null; then
    echo -e "${GREEN}✓${NC} Vous pouvez utiliser Docker sans sudo"
else
    echo -e "${RED}✗${NC} Vous ne pouvez pas utiliser Docker sans sudo"
    echo ""
    echo "   Solutions possibles:"
    echo "   1. Ajoutez votre utilisateur au groupe docker:"
    echo "      ${YELLOW}sudo usermod -aG docker $CURRENT_USER${NC}"
    echo ""
    echo "   2. Reconnectez-vous ou exécutez:"
    echo "      ${YELLOW}newgrp docker${NC}"
    echo ""
    echo "   3. Redémarrez votre session"
fi
echo ""

# 7. Vérifier si le dossier /tmp/projects existe
echo "7. Vérification du dossier des projets..."
if [ -d "/tmp/projects" ]; then
    echo -e "${GREEN}✓${NC} Le dossier /tmp/projects existe"
    ls -la /tmp/projects
else
    echo -e "${YELLOW}⚠${NC} Le dossier /tmp/projects n'existe pas"
    echo "   Création du dossier..."
    mkdir -p /tmp/projects
    echo -e "${GREEN}✓${NC} Dossier créé: /tmp/projects"
fi
echo ""

# 8. Résumé
echo "======================================"
echo "  RÉSUMÉ"
echo "======================================"

CAN_RUN_DOCKER=false
if docker ps &> /dev/null; then
    CAN_RUN_DOCKER=true
fi

if [ "$CAN_RUN_DOCKER" = true ]; then
    echo -e "${GREEN}✓ Votre système est prêt pour Docker Wrapper !${NC}"
    echo ""
    echo "Vous pouvez maintenant lancer l'application avec:"
    echo "  ${GREEN}docker-compose up -d${NC}"
else
    echo -e "${RED}✗ Configuration incomplète${NC}"
    echo ""
    echo "Veuillez suivre les recommandations ci-dessus."
fi
echo ""
