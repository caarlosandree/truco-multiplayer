#!/usr/bin/env bash
# Baixa os pacotes CC0 da Kenney e gera OGG + MP3 com os nomes usados pelo app.
set -euo pipefail
cd "$(dirname "$0")/../.."
out=frontend/src/assets/sfx
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# Links "Download" atuais (conferidos em kenney.nl/assets/casino-audio e /interface-sounds).
curl -fsSL -o "$tmp/casino.zip" "https://kenney.nl/media/pages/assets/casino-audio/2472606a04-1721639069/kenney_casino-audio.zip"
curl -fsSL -o "$tmp/ui.zip" "https://kenney.nl/media/pages/assets/interface-sounds/fa43c1dd4d-1677589452/kenney_interface-sounds.zip"
unzip -q "$tmp/casino.zip" -d "$tmp/casino"
unzip -q "$tmp/ui.zip" -d "$tmp/ui"

pick() { # nome-final arquivo-fonte
  ffmpeg -loglevel error -y -i "$2" -ac 1 -ar 44100 -c:a libvorbis -q:a 3 "$out/$1.ogg"
  ffmpeg -loglevel error -y -i "$2" -ac 1 -ar 44100 -c:a libmp3lame -b:a 64k "$out/$1.mp3"
}

mkdir -p "$out"
pick shuffle     "$tmp/casino/Audio/card-shuffle.ogg"
pick deal        "$tmp/casino/Audio/card-fan-1.ogg"
pick card        "$tmp/casino/Audio/card-place-1.ogg"
pick cardCovered "$tmp/casino/Audio/card-slide-1.ogg"
pick burn        "$tmp/casino/Audio/card-shove-1.ogg"
pick flip        "$tmp/casino/Audio/card-slide-3.ogg"
pick trickWon    "$tmp/casino/Audio/chips-collide-1.ogg"
pick truco       "$tmp/ui/Audio/confirmation_002.ogg"
pick accept      "$tmp/ui/Audio/confirmation_001.ogg"
pick run         "$tmp/ui/Audio/error_004.ogg"
pick point       "$tmp/casino/Audio/chips-stack-1.ogg"
pick yourTurn    "$tmp/ui/Audio/bong_001.ogg"
pick timeLow     "$tmp/ui/Audio/tick_002.ogg"
pick victory     "$tmp/ui/Audio/maximize_006.ogg"
pick defeat      "$tmp/ui/Audio/minimize_006.ogg"
du -ch "$out"/*.ogg | tail -1
