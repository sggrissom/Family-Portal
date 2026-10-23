#!/usr/bin/env bash
# Builds whisper.cpp and fetches its models into DEST, for video transcripts.
# Run it on the server: a build from a newer distro needs a newer glibc than
# the boxes have. The build is portable (every x86 CPU variant, picked at run
# time), so staging and production can share it. Needs git, cmake and g++.
# Safe to re-run: verified models and a working build are kept.
set -euo pipefail

dest="${1:?usage: install-speech.sh DEST}"
whisper_tag="v1.9.5"
whisper_commit="d1be6fde11ac6e0407606b4e42fe72d34add8037"

files=(
  "ggml-small.en.bin https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.en.bin c6138d6d58ecc8322097e0f987c32f1be8bb0a18532a3f88f734d1bbf9c41e5d"
  "ggml-silero-v6.2.0.bin https://huggingface.co/ggml-org/whisper-vad/resolve/main/ggml-silero-v6.2.0.bin 2aa269b785eeb53a82983a20501ddf7c1d9c48e33ab63a41391ac6c9f7fb6987"
)

verified() {
  [ -f "$1" ] && echo "$2  $1" | sha256sum --check --status
}

mkdir -p "$dest"
cd "$dest"
for entry in "${files[@]}"; do
  read -r name url sha <<<"$entry"
  target="$dest/$name"
  if verified "$target" "$sha"; then
    echo "ok       $name"
    continue
  fi
  echo "fetching $name"
  curl -fsSL -o "$target.part" "$url"
  echo "$sha  $target.part" | sha256sum --check --status || { echo "checksum mismatch: $name" >&2; rm -f "$target.part"; exit 1; }
  mv "$target.part" "$target"
done

stamp="$dest/bin/.built-$whisper_commit"
if [ ! -f "$stamp" ]; then
  src="$(mktemp -d)"
  trap 'rm -rf "$src"' EXIT
  git -c advice.detachedHead=false clone -q --depth 1 --branch "$whisper_tag" https://github.com/ggml-org/whisper.cpp "$src/whisper.cpp"
  got="$(git -C "$src/whisper.cpp" rev-parse HEAD)"
  [ "$got" = "$whisper_commit" ] || { echo "tag $whisper_tag is $got, want $whisper_commit" >&2; exit 1; }
  cmake -S "$src/whisper.cpp" -B "$src/build" -DCMAKE_BUILD_TYPE=Release \
    -DGGML_NATIVE=OFF -DGGML_BACKEND_DL=ON -DGGML_CPU_ALL_VARIANTS=ON \
    -DWHISPER_SDL2=OFF -DWHISPER_BUILD_TESTS=OFF -DWHISPER_BUILD_SERVER=OFF \
    -DCMAKE_INSTALL_LIBDIR=lib '-DCMAKE_INSTALL_RPATH=$ORIGIN/../lib' >/dev/null
  cmake --build "$src/build" -j"$(nproc)" >/dev/null
  rm -rf "$dest/bin" "$dest/lib"
  cmake --install "$src/build" --prefix "$src/install" >/dev/null
  mkdir -p "$dest/bin" "$dest/lib"
  cp "$src/install/bin/whisper-cli" "$dest/bin/"
  cp -P "$src/install/lib/"*.so* "$dest/lib/"
  # ggml looks for its CPU variants beside the executable.
  find "$src/build" -name 'libggml-cpu-*.so' -exec cp {} "$dest/bin/" \;
  cp "$src/whisper.cpp/samples/jfk.wav" "$dest/bin/jfk.wav"
  touch "$stamp"
fi

out="$("$dest/bin/whisper-cli" -m "$dest/ggml-small.en.bin" -f "$dest/bin/jfk.wav" -t 2 -np \
  --vad -vm "$dest/ggml-silero-v6.2.0.bin" 2>&1)"
if ! grep -qi "fellow americans" <<<"$out"; then
  echo "whisper-cli did not transcribe the sample:" >&2
  echo "$out" | tail -20 >&2
  exit 1
fi
echo "ok       whisper-cli $whisper_tag transcribes the sample"
