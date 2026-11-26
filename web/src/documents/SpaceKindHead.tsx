import type { ReactNode } from "react";
import { Count } from "../ui/Count";
import { GroupHead } from "../ui/GroupHead";
import { Icon } from "../ui/Icon";
import { Segment, Segmented } from "../ui/Segmented";
import { Option, Select } from "../ui/Select";
import css from "./SpaceKindHead.module.css";
import { Text } from "../ui/Text";
import { Spacer } from "../ui/Spacer";

// The segment for every state: no state is named this.
const everyStand = "(all)";

type SpaceKindHeadProps = {
  title: string;
  count: ReactNode; // how many (of them, how many a search found)
  states: [string, number][]; // where one can stand, and how many do
  stand: string; // "" for all
  setStand: (s: string) => void;
  sorts: [string, string][]; // how it can be sorted, and what that's called
  sort: string;
  setSort: (s: string) => void;
};

// SpaceKindHead is a kind's list's heading in a space: its title and how
// many, where they stand (one picked keeps the list to it), and how it's
// sorted.
export function SpaceKindHead({ title, count, states, stand, setStand, sorts, sort, setSort }: SpaceKindHeadProps) {
  return (
    <GroupHead className={css.head}>
      {title} <Count>{count}</Count>
      {states.length > 0 && (
        <Segmented size="small" className={css.stands} value={stand || everyStand} onChange={(v) => setStand(v === everyStand ? "" : v)} aria-label={`Filter ${title} by status`}>
          <Segment value={everyStand}>All</Segment>
          {states.map(([s, n]) => (
            <Segment key={s} value={s}>
              {s} {n}
            </Segment>
          ))}
        </Segmented>
      )}
      <Spacer />
      <Text as="label" dim className={css.sort}>
        <Icon name="list" size={12} />
        <Select size="tiny" value={sort} onChange={setSort} aria-label={`Sort ${title}`}>
          {sorts.map(([v, label]) => (
            <Option key={v} value={v}>
              {label}
            </Option>
          ))}
        </Select>
      </Text>
    </GroupHead>
  );
}
