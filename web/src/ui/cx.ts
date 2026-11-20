// cx is a className from several: a global class and a module's, or a
// modifier that's there only when its condition holds.
//
//   <li className={cx("row", css.row, open && css.open)}>
export function cx(...names: (string | false | null | undefined)[]): string {
  return names.filter(Boolean).join(" ");
}
