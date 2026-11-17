export interface Shape {
  area(): number;
}

export class Base {
  id = "b";
  describe(): string {
    return this.id;
    //          ^ resolved :id "a member of Base"
  }
}

export class Store {
  get(key: string): string {
    return key;
  }
}

export enum Colour {
  Red,
  Green = "g",
}

export class Square extends Base implements Shape {
  private size: number;
  constructor(private readonly store: Store, side: number) {
    super();
    this.size = side;
    //   ^ resolved :size "a member of Square"
  }
  area(): number {
    return this.size * this.store.get("k").length + super.describe().length;
    //          ^ resolved :size
    //                       ^ resolved :store
    //                             ^ resolved :get "by the type written on this.store"
    //                                                    ^ resolved :describe
  }
  label(): string {
    return this.describe() + Colour.Red;
    //          ^ resolved :describe
    //                              ^ resolved :Red "a member of Colour"
  }
}

export function measure(s: Shape, q: Square) {
  const made = new Square(new Store(), 2);
  return s.area() + q.label().length + made.area();
  //       ^ resolved :area "by the type written on s"
  //                  ^ resolved :label "by the type written on q"
  //                                        ^ resolved :area "by the type written on made"
}
