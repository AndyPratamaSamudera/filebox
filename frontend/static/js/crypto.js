/**
 * Lightweight encoding helpers for FileBox.
 * Server-side disk encryption replaces the previous client-side RSA/AES code.
 */

export function bytesToBase64(bytes) {
  const bin = Array.from(bytes, (b) => String.fromCharCode(b)).join('');
  return btoa(bin);
}

export function base64ToBytes(base64) {
  const bin = atob(base64);
  return new Uint8Array(bin.length).map((_, i) => bin.charCodeAt(i));
}

export function stringToBytes(str) {
  return new TextEncoder().encode(str);
}

export function bytesToString(bytes) {
  return new TextDecoder().decode(bytes);
}

export function randomBase64(byteLength) {
  return bytesToBase64(crypto.getRandomValues(new Uint8Array(byteLength)));
}

export function chunkRanges(fileSize, chunkSize) {
  const ranges = [];
  for (let start = 0; start < fileSize; start += chunkSize) {
    ranges.push({ start, end: Math.min(fileSize, start + chunkSize) });
  }
  return ranges;
}
