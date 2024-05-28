"use client";

import React, { useState, useEffect } from "react";
import dynamic from "next/dynamic";

const DynamicReactSwagger = dynamic(() => import("./react-swagger"), {
  ssr: false,
  loading: () => <div>Loading...</div>,
});

export default function IndexPage() {
  const [spec, setSpec] = useState(null);

  useEffect(() => {
    async function fetchSpecs() {
      const response = await fetch("/api/getSwaggerDocs");

      if (response.ok) {
        const fetchedSpec = await response.json();
        setSpec(fetchedSpec);
      } else {
        console.error("Failed to fetch API docs");
      }
    }

    fetchSpecs();
  }, []);

  if (!spec) return <div>Loading...</div>;

  return (
    <section className="container">
      <DynamicReactSwagger spec={spec} />
    </section>
  );
}
