// setClock moves the page's clock to when the stories' missions were
// taken (at, as fixtures/taken.json has it), and lets it run from there:
// a mission that waited two minutes then has waited two minutes now, not
// however long it's been since.
export function setClock(at: string) {
  const Real = Date;
  const offset = new Real(at).getTime() - Real.now();
  if (!Number.isFinite(offset)) return;
  class Shifted extends Real {
    constructor(...args: unknown[]) {
      if (args.length === 0) super(Real.now() + offset);
      else super(...(args as [number]));
    }
    static now() {
      return Real.now() + offset;
    }
  }
  globalThis.Date = Shifted as DateConstructor;
}
