import { useEffect, useState } from "react";
import { Link, Navigate, useLocation, useNavigate, useParams, useSearchParams } from "react-router";
import { docAddress, docTo, filesTo, GROUND, openFilePage, pageAddress } from "../api";
import { DocPage } from "../files/DocPage";
import { PagePage } from "../pages/Pages";
import { ErrorCard, humanError, Loading } from "../ui/kit";
import { useWorkspace } from "../workspace/store";

// A document's one address: /documents/<repo>/<path>, read first, edited
// there (?edit) as the page tied to the file, made the first time, its
// version in a capsule with ?in=<capsule>. A local page is
// /documents/local/<id>. The addresses that were (/docs?ref=,
// /pages/<id>, /pages/open?ref=) lead here.

// DocumentAt is /documents/<repo>/<path>: the document, read or edited.
export function DocumentAt() {
  const params = useParams();
  const [q] = useSearchParams();
  const repo = params.repo ?? "";
  const path = params["*"] ?? "";
  const capsule = q.get("in") ?? "";
  const ref = `${repo}${capsule ? `/${capsule}` : ""}:${path}`;
  if (q.has("edit")) return <EditAt key={ref} docRef={ref} page={q.get("edit") || undefined} />;
  return <DocPage key={ref} docRef={ref} />;
}

// EditAt is a document edited at its address: the page tied to it, given
// (?edit=p8), else found or made for it. One a mission records is that
// mission's document, edited there.
function EditAt({ docRef, page }: { docRef: string; page?: string }) {
  const navigate = useNavigate();
  const [id, setId] = useState(page);
  const [error, setError] = useState<string>();
  useEffect(() => {
    if (page) {
      setId(page);
      return;
    }
    let stale = false;
    openFilePage(docRef).then(
      (r) => {
        if (stale) return;
        if (r.mission) navigate(`/missions/${r.mission.slug}/artifact?${new URLSearchParams({ ref: r.mission.ref })}`, { replace: true });
        else navigate(docAddress(docRef, r.id), { replace: true }); // its page, said, so a reload keeps it
      },
      (e: Error) => !stale && setError(humanError(e.message)),
    );
    return () => {
      stale = true;
    };
  }, [docRef, page, navigate]);
  if (error) {
    const at = docRef.indexOf(":");
    const [repo, capsule] = docRef.slice(0, at).split("/");
    return (
      <div className="page narrow">
        <ErrorCard error={error} title="Couldn't open it to edit" />
        <p>
          <Link to={docTo(docRef)}>Read it</Link> · <a href={filesTo(repo, capsule || GROUND, docRef.slice(at + 1))}>Open it in Files</a>
        </p>
      </div>
    );
  }
  if (!id) return <Loading what="Opening it to edit…" />;
  return <PagePage key={id} id={id} back={{ to: docTo(docRef), label: "Read it" }} />;
}

// LocalPageAt is /documents/local/<id>: a page that isn't in a space.
export function LocalPageAt() {
  const { id = "" } = useParams();
  return <PagePage key={id} id={id} back={{ to: "/documents", label: "Documents" }} />;
}

// DocsAddress is /docs?ref=, as it was: the document's address now.
export function DocsAddress() {
  const [q] = useSearchParams();
  const { hash } = useLocation();
  const ref = q.get("ref") ?? "";
  return <Navigate to={ref ? docTo(ref) + hash : "/documents"} replace />;
}

// OpenAddress is /pages/open?ref=, as it was: the document, edited.
export function OpenAddress() {
  const [q] = useSearchParams();
  const ref = q.get("ref") ?? "";
  return <Navigate to={ref ? docAddress(ref, true) : "/documents"} replace />;
}

// PageAddress is /pages/<id>, as it was: where the page is now, the
// document it's tied to or Local.
export function PageAddress() {
  const pages = useWorkspace((w) => w.pages);
  const { id = "" } = useParams();
  const { hash } = useLocation();
  const p = pages?.find((x) => x.id === id);
  return <Navigate to={pageAddress(p ?? { id }) + hash} replace />;
}
