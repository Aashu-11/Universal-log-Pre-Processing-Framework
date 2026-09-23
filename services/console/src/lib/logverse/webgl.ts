/** Cheap, synchronous WebGL capability probe — used to decide whether to
 * mount the R3F <Canvas> at all or fall back to WebGLFallback. Creates and
 * immediately discards a throwaway canvas; does not touch the DOM. */
export function isWebglAvailable(): boolean {
  try {
    const canvas = document.createElement("canvas");
    const ctx = canvas.getContext("webgl2") || canvas.getContext("webgl") || canvas.getContext("experimental-webgl");
    return !!ctx;
  } catch {
    return false;
  }
}
