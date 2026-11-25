// ReaderEarlier says how many threads were made on a document before it
// was edited as a page: they're kept, and listed by ws show.
export function ReaderEarlier({ count, docRef }: { count: number; docRef: string }) {
  return (
    <p className="dim">
      {count === 1 ? "A thread was" : `${count} threads were`} made on this file before it was edited as a page. {count === 1 ? "It's" : "They're"} kept: <span className="mono">ws show {docRef}</span> lists {count === 1 ? "it" : "them"}. Ask and change on its page now.
    </p>
  );
}
