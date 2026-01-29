import { useState } from "react";
import { Badge } from "@web-ui/badge";
import { Button } from "@web-ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@web-ui/card";

function App() {
  const [count, setCount] = useState(0);

  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="mx-auto max-w-3xl px-6 py-14">
        <h1 className="text-3xl font-semibold tracking-tight">
          UI Playground
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Fast Vite sandbox for shadcn components in <code>apps/web</code>.
        </p>

        <div className="mt-8 grid gap-4 md:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                Buttons <Badge variant="outline">ui</Badge>
              </CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap gap-3">
              <Button onClick={() => setCount((c) => c + 1)}>
                Clicked {count}
              </Button>
              <Button variant="secondary">Secondary</Button>
              <Button variant="outline">Outline</Button>
              <Button variant="ghost">Ghost</Button>
              <Button variant="destructive">Destructive</Button>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Badges</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap gap-2">
              <Badge>Default</Badge>
              <Badge variant="secondary">Secondary</Badge>
              <Badge variant="outline">Outline</Badge>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}

export default App;
