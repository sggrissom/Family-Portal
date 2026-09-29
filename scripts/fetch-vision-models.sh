#!/usr/bin/env bash
# Downloads the family-vision models and ONNX Runtime into DEST, verifying
# each file's checksum. Safe to re-run: verified files are skipped.
set -euo pipefail

dest="${1:?usage: fetch-vision-models.sh DEST}"
clip="https://huggingface.co/Xenova/clip-vit-base-patch32/resolve/d15189d7028b43f1d3e65039190477f6af591c2a"
minilm="https://huggingface.co/Xenova/all-MiniLM-L6-v2/resolve/751bff37182d3f1213fa05d7196b954e230abad9"
ort_version="1.29.1"
ort_url="https://github.com/microsoft/onnxruntime/releases/download/v${ort_version}/onnxruntime-linux-x64-${ort_version}.tgz"

files=(
  "clip/visual.onnx $clip/onnx/vision_model.onnx fd6e1402a588279d1723c7534d4bcba5bc0b14b47dfab0e46f8c47b8270d7d40"
  "clip/textual.onnx $clip/onnx/text_model.onnx 3f6571f5bad13a97c469c1622e1cfc4d9aef78b79fdbfcff804ca357bfada8cc"
  "clip/vocab.json $clip/vocab.json 5047b556ce86ccaf6aa22b3ffccfc52d391ea4accdab9c2f2407da5b742d4363"
  "clip/merges.txt $clip/merges.txt 9fd691f7c8039210e0fced15865466c65820d09b63988b0174bfe25de299051a"
  "minilm/model.onnx $minilm/onnx/model.onnx 759c3cd2b7fe7e93933ad23c4c9181b7396442a2ed746ec7c1d46192c469c46e"
  "minilm/vocab.txt $minilm/vocab.txt 07eced375cec144d27c900241f3e339478dec958f92fddbc551f295c992038a3"
)
ort_sha="ff3de2363a0cb79e5ee79ef59594eae04930c6d7a6b6654c399e7977a5404a4b"

verified() {
  [ -f "$1" ] && echo "$2  $1" | sha256sum --check --status
}

mkdir -p "$dest/clip" "$dest/minilm"

for entry in "${files[@]}"; do
  read -r path url sha <<<"$entry"
  target="$dest/$path"
  if verified "$target" "$sha"; then
    echo "ok       $path"
    continue
  fi
  echo "fetching $path"
  curl -fsSL -o "$target.part" "$url"
  echo "$sha  $target.part" | sha256sum --check --status || { echo "checksum mismatch: $path" >&2; rm -f "$target.part"; exit 1; }
  mv "$target.part" "$target"
done

lib="$dest/libonnxruntime.so"
if verified "$lib" "$ort_sha"; then
  echo "ok       libonnxruntime.so"
else
  echo "fetching onnxruntime $ort_version"
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT
  curl -fsSL "$ort_url" | tar xz -C "$tmp"
  cp -L "$tmp/onnxruntime-linux-x64-${ort_version}/lib/libonnxruntime.so.${ort_version}" "$lib.part"
  echo "$ort_sha  $lib.part" | sha256sum --check --status || { echo "checksum mismatch: libonnxruntime.so" >&2; rm -f "$lib.part"; exit 1; }
  mv "$lib.part" "$lib"
fi
