import type { ReactNode } from "react";
import css from "./SettingsMine.module.css";

// SettingsMine is a list of what's yours alone: what your file sets, what
// this browser keeps. A SettingsMineRow each.
export function SettingsMine({ children }: { children: ReactNode }) {
  return <div>{children}</div>;
}

// SettingsMineRow is one of them: what it is, what it's set to, and the
// way back where there is one.
export function SettingsMineRow({ children }: { children: ReactNode }) {
  return <div className={css.row}>{children}</div>;
}

// SettingsMineWhat is which setting it is, over its key.
export function SettingsMineWhat({ children }: { children: ReactNode }) {
  return <div className={css.what}>{children}</div>;
}

// SettingsMineValue is what it's set to, over what it would be otherwise.
export function SettingsMineValue({ children }: { children: ReactNode }) {
  return <div className={css.value}>{children}</div>;
}
