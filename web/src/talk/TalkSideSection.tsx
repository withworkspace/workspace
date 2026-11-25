import type { ReactNode } from "react";

// TalkSideSection is a section of what's come of a conversation, under
// its heading (tip, said when it's pointed at). Its look is TalkSide's,
// the column it's in.
export function TalkSideSection({ title, tip, children }: { title: string; tip?: string; children: ReactNode }) {
  return (
    <section>
      <h3 title={tip}>{title}</h3>
      {children}
    </section>
  );
}
