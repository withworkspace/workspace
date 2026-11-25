import { useCallback, useEffect, useRef, useState } from "react";
import { readChange, readTold, type State, type Told } from "./tweaks";

// Tweaks: what a version's page says can be changed about it, drawn
// beside its frame, and not by the page: a tray of its presets and a
// control for each tweak (TweaksTray.tsx). The page declares them with
// ws.tweaks (which the server writes into every frame), tells this page
// over postMessage, and is told back what to set. The frame's origin is
// its own, so its messages are known by the window they come from, and
// read as data (tweaks.ts).

// FrameTweaks is a frame's tweaks, for the page around it: frame is the
// iframe's ref, told what its page declared and the state it's in (none
// until it says, and for a page that declares none), and set changes some
// of them.
export interface FrameTweaks {
  frame: (el: HTMLIFrameElement | null) => void;
  told: Told | undefined;
  set: (some: State) => void;
}

// useTweaks is the tweaks of the frame that's showing what: a version, at
// a run of it (a3.v2#0). What a frame said is kept with what it was
// showing, so the moment the page asks for another (a version stepped to,
// a reload), there's nothing told: not the last frame's, a render late.
export function useTweaks(what: string): FrameTweaks {
  const [el, setEl] = useState<HTMLIFrameElement | null>(null);
  const [said, setSaid] = useState<{ what: string; told: Told }>();
  const showing = useRef(what);
  showing.current = what;
  useEffect(() => {
    if (!el) return;
    const onMessage = (e: MessageEvent) => {
      if (e.source !== el.contentWindow) return;
      const all = readTold(e.data);
      if (all) return setSaid({ what: showing.current, told: all });
      setSaid((s) => {
        const state = s && s.what === showing.current ? readChange(s.told, e.data) : undefined;
        return s && state ? { what: s.what, told: { ...s.told, state } } : s;
      });
    };
    // The page says what it declares as it loads. Asked too, now and once
    // it's loaded, for one that said so before this was listening.
    const hello = () => el.contentWindow?.postMessage({ ws: "tweaks:hello" }, "*");
    window.addEventListener("message", onMessage);
    el.addEventListener("load", hello);
    hello();
    return () => {
      window.removeEventListener("message", onMessage);
      el.removeEventListener("load", hello);
    };
  }, [el]);
  const set = useCallback((some: State) => el?.contentWindow?.postMessage({ ws: "tweaks:set", state: some }, "*"), [el]);
  return { frame: setEl, told: said?.what === what ? said.told : undefined, set };
}

// hasTweaks is whether a frame's page declared any.
export const hasTweaks = (told: Told | undefined): told is Told => !!told && told.tweaks.length > 0;
