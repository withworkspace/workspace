export const LIMIT = 3;

export function count(items: string[]): number {
  let n = 0;
  for (const item of items) {
    if (item === "") continue;
    //  ^ resolved :item "local"
    n += later(item);
    //   ^ resolved :later "in this file"
  }
  const { length, at: first } = items;
  console.log(length, first);
  //          ^ resolved :length "local"
  //                  ^ resolved :first "local"
  // <- builtin
  return n > LIMIT ? n : Math.max(n, 0);
  //         ^ resolved :LIMIT "in this file"
  //                     ^ builtin
}

const double = (x: number) => x * 2;
//                            ^ resolved :x "local"

function later(s: string): number {
  return double(s.length) + hoisted();
  //     ^ resolved :double "in this file"
  //                        ^ resolved :hoisted "local"
  function hoisted() {
    return 1;
  }
}

export function shadow() {
  const LIMIT = "four";
  return LIMIT + missing;
  //     ^ resolved :LIMIT "local"
  //             ^ unresolved
}
