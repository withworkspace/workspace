import { useEffect, useState } from "react";
import { Printed } from "../../ui/Printed";

// VerifyTranscript is a bundle's text, read from where it's served.
export function VerifyTranscript({ url }: { url: string }) {
  const [text, setText] = useState<string>();
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let gone = false;
    setText(undefined);
    setFailed(false);
    fetch(url)
      .then((res) => (res.ok ? res.text() : Promise.reject(new Error(String(res.status)))))
      .then((t) => !gone && setText(t))
      .catch(() => !gone && setFailed(true));
    return () => {
      gone = true;
    };
  }, [url]);
  if (failed) return <p>Could not load file.</p>;
  return <Printed>{text ?? " "}</Printed>;
}
