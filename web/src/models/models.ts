import { getModels, type ModelInfo } from "../api";

// What a harness lists is asked of its CLI, which is slow: asked once for
// all the roles on it, while the page is open.
const listed = new Map<string, Promise<ModelInfo[]>>();

export function modelsOf(harness: string): Promise<ModelInfo[]> {
  let p = listed.get(harness);
  if (!p) {
    p = getModels(harness);
    listed.set(harness, p);
    p.catch(() => listed.delete(harness));
  }
  return p;
}

// forgetModels has them asked again, the next time: on coming to the page.
export const forgetModels = () => listed.clear();
