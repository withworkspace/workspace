// rowEl is where a row of a review's diff is drawn, by its key: a line, a
// gap or a note (ui/DiffLines says which with data-row), a card among
// them, or a file's head (#row-<key>).
export const rowEl = (key: string): HTMLElement | null => document.querySelector<HTMLElement>(`[data-row="${CSS.escape(key)}"]`) ?? document.getElementById(`row-${key}`);
