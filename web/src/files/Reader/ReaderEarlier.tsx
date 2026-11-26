import { Text } from "../../ui/Text";
// ReaderEarlier says how many threads were made on a document before it
// was edited as a page: they're kept, and listed by ws show.
export function ReaderEarlier({ count, docRef }: { count: number; docRef: string }) {
  return (
    <Text as="p" dim>
      {count === 1 ? "A thread was" : `${count} threads were`} made on this file before it became a page.{" "}
      {count === 1 ? "It's" : "They're"} kept: <Text mono>ws show {docRef}</Text> lists{" "}
      {count === 1 ? "it" : "them"}.
    </Text>
  );
}
