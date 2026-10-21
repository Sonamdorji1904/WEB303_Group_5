"use client";
export default function Error({ reset }: { reset: () => void }) {
  return <div className="card mx-auto mt-12 max-w-lg p-8 text-center"><h2 className="text-xl font-bold">Something went wrong</h2><p className="mt-2 text-sm text-slate-500">Please try the page again.</p><button className="btn-primary mt-5" onClick={reset}>Try again</button></div>;
}
