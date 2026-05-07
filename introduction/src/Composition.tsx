import type {CSSProperties, ReactNode} from "react";
import {
  AbsoluteFill,
  Easing,
  Sequence,
  interpolate,
  useCurrentFrame,
  useVideoConfig,
} from "remotion";

const fullSpec = {
  bytes: "44,250 bytes",
  lines: "1,450 lines",
};

const miniSpec = {
  bytes: "14,837 bytes",
  lines: "514 lines",
};

const endpoints = [
  ["GET", "/me", "current user"],
  ["GET", "/planets", "list planets"],
  ["GET", "/planets/{planetId}", "planet detail"],
  ["POST", "/auth/token", "auth token"],
] as const;

const shell: CSSProperties = {
  background:
    "radial-gradient(circle at 20% 15%, rgba(20, 184, 166, 0.22), transparent 32%), radial-gradient(circle at 80% 20%, rgba(249, 115, 22, 0.18), transparent 34%), linear-gradient(135deg, #07111f 0%, #0f172a 44%, #111827 100%)",
  color: "#f8fafc",
  fontFamily:
    'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
  overflow: "hidden",
};

const panel: CSSProperties = {
  border: "1px solid rgba(148, 163, 184, 0.22)",
  background: "rgba(15, 23, 42, 0.76)",
  boxShadow: "0 30px 80px rgba(2, 6, 23, 0.4)",
};

const codeFont: CSSProperties = {
  fontFamily:
    'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace',
};

const ease = Easing.bezier(0.16, 1, 0.3, 1);

const clampFade = (
  frame: number,
  input: [number, number],
  output: [number, number],
) =>
  interpolate(frame, input, output, {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
    easing: ease,
  });

const Scene = ({
  children,
  accent = "#14b8a6",
}: {
  children: ReactNode;
  accent?: string;
}) => {
  const frame = useCurrentFrame();
  const {fps} = useVideoConfig();
  const opacity = Math.min(
    clampFade(frame, [0, 0.45 * fps], [0, 1]),
    clampFade(frame, [4.2 * fps, 5 * fps], [1, 0]),
  );
  const y = interpolate(frame, [0, 0.8 * fps], [28, 0], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
    easing: ease,
  });

  return (
    <AbsoluteFill style={{...shell, opacity}}>
      <div
        style={{
          position: "absolute",
          inset: 0,
          background: `linear-gradient(90deg, ${accent} 0%, transparent 34%, transparent 100%)`,
          opacity: 0.12,
        }}
      />
      <div
        style={{
          position: "absolute",
          inset: "46px 56px",
          transform: `translateY(${y}px)`,
        }}
      >
        {children}
      </div>
    </AbsoluteFill>
  );
};

const Eyebrow = ({children}: {children: ReactNode}) => (
  <div
    style={{
      color: "#5eead4",
      fontSize: 18,
      fontWeight: 800,
      letterSpacing: 0,
      marginBottom: 16,
      textTransform: "uppercase",
    }}
  >
    {children}
  </div>
);

const Headline = ({children}: {children: ReactNode}) => (
  <div
    style={{
      fontSize: 70,
      lineHeight: 1.01,
      fontWeight: 900,
      letterSpacing: 0,
      maxWidth: 980,
    }}
  >
    {children}
  </div>
);

const Subline = ({children}: {children: ReactNode}) => (
  <div
    style={{
      color: "#cbd5e1",
      fontSize: 25,
      lineHeight: 1.28,
      marginTop: 20,
      maxWidth: 840,
      fontWeight: 550,
    }}
  >
    {children}
  </div>
);

const Terminal = ({
  children,
  style,
}: {
  children: ReactNode;
  style?: CSSProperties;
}) => (
  <div
    style={{
      ...panel,
      ...codeFont,
      borderRadius: 18,
      padding: "24px 28px",
      color: "#dbeafe",
      fontSize: 28,
      lineHeight: 1.45,
      ...style,
    }}
  >
    <div style={{display: "flex", gap: 8, marginBottom: 18}}>
      {["#ef4444", "#f59e0b", "#22c55e"].map((color) => (
        <div
          key={color}
          style={{width: 14, height: 14, borderRadius: 999, background: color}}
        />
      ))}
    </div>
    {children}
  </div>
);

const StatCard = ({
  label,
  value,
  note,
  color,
}: {
  label: string;
  value: string;
  note: string;
  color: string;
}) => (
  <div
    style={{
      ...panel,
      borderRadius: 18,
      padding: 24,
      width: 400,
    }}
  >
    <div style={{color, fontSize: 18, fontWeight: 800, marginBottom: 14}}>
      {label}
    </div>
    <div style={{fontSize: 42, fontWeight: 900, marginBottom: 8}}>{value}</div>
    <div style={{color: "#94a3b8", fontSize: 22}}>{note}</div>
  </div>
);

const IntroScene = () => (
  <Scene>
    <div style={{display: "grid", gridTemplateColumns: "1.1fr 0.9fr", gap: 56}}>
      <div style={{alignSelf: "center"}}>
        <Eyebrow>openapi-extract</Eyebrow>
        <Headline>Stop feeding entire OpenAPI specs to AI agents.</Headline>
        <Subline>
          Extract the endpoints you need, then give your agent a focused mini
          spec.
        </Subline>
      </div>
      <Terminal style={{alignSelf: "center"}}>
        <div style={{color: "#94a3b8"}}>openapi.yaml</div>
        <div>paths:</div>
        <div style={{color: "#fca5a5"}}>  /auth/token</div>
        <div style={{color: "#fca5a5"}}>  /celestial-bodies</div>
        <div style={{color: "#fca5a5"}}>  /me</div>
        <div style={{color: "#fca5a5"}}>  /planets</div>
        <div style={{color: "#fca5a5"}}>  /planets/&#123;planetId&#125;</div>
        <div style={{color: "#94a3b8"}}>  ...and more context</div>
      </Terminal>
    </div>
  </Scene>
);

const ProblemScene = () => {
  const frame = useCurrentFrame();
  const {fps} = useVideoConfig();
  const full = clampFade(frame, [0.5 * fps, 1.8 * fps], [0, 100]);
  const mini = clampFade(frame, [1.4 * fps, 2.7 * fps], [0, 34]);

  return (
    <Scene accent="#f97316">
      <Eyebrow>The context problem</Eyebrow>
      <Headline>Most prompts only need one slice of the API.</Headline>
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "1fr 1fr",
          gap: 42,
          marginTop: 48,
        }}
      >
        <div>
          <StatCard
            label="FULL SPEC"
            value={fullSpec.bytes}
            note={fullSpec.lines}
            color="#fb7185"
          />
          <div
            style={{
              height: 28,
              width: 455,
              background: "rgba(148, 163, 184, 0.2)",
              borderRadius: 999,
              marginTop: 28,
              overflow: "hidden",
            }}
          >
            <div
              style={{
                width: `${full}%`,
                height: "100%",
                background: "#fb7185",
                borderRadius: 999,
              }}
            />
          </div>
        </div>
        <div>
          <StatCard
            label="MINI SPEC"
            value={miniSpec.bytes}
            note={`${miniSpec.lines} for GET /planets/{planetId}`}
            color="#2dd4bf"
          />
          <div
            style={{
              height: 28,
              width: 455,
              background: "rgba(148, 163, 184, 0.2)",
              borderRadius: 999,
              marginTop: 28,
              overflow: "hidden",
            }}
          >
            <div
              style={{
                width: `${mini}%`,
                height: "100%",
                background: "#2dd4bf",
                borderRadius: 999,
              }}
            />
          </div>
        </div>
      </div>
      <div style={{fontSize: 40, fontWeight: 900, marginTop: 30}}>
        About 66% smaller in the Scalar Galaxy example.
      </div>
    </Scene>
  );
};

const ListScene = () => (
  <Scene>
    <div style={{display: "grid", gridTemplateColumns: "0.95fr 1.05fr", gap: 56}}>
      <div style={{alignSelf: "center"}}>
        <Eyebrow>Discover endpoints</Eyebrow>
        <Headline>List operations without opening the whole YAML.</Headline>
        <Subline>
          Great for agents that need a clean menu before choosing what to
          extract.
        </Subline>
      </div>
      <Terminal>
        <div>
          <span style={{color: "#5eead4"}}>$</span> openapi-extract list
          openapi.yaml
        </div>
        <div style={{height: 16}} />
        {endpoints.map(([method, path, label], index) => (
          <div
            key={path}
            style={{
              display: "grid",
              gridTemplateColumns: "98px 1fr auto",
              gap: 18,
              opacity: 1 - index * 0.08,
            }}
          >
            <span style={{color: method === "GET" ? "#5eead4" : "#fbbf24"}}>
              {method}
            </span>
            <span>{path}</span>
            <span style={{color: "#94a3b8"}}>{label}</span>
          </div>
        ))}
      </Terminal>
    </div>
  </Scene>
);

const ExtractScene = () => (
  <Scene accent="#22c55e">
    <Eyebrow>Extract only what matters</Eyebrow>
    <Headline>Save or copy a mini OpenAPI spec in one command.</Headline>
    <div
      style={{
        display: "grid",
        gridTemplateColumns: "1.1fr 0.9fr",
        gap: 42,
        marginTop: 44,
      }}
    >
      <Terminal>
        <div>
          <span style={{color: "#5eead4"}}>$</span> openapi-extract extract
          openapi.yaml \
        </div>
        <div style={{paddingLeft: 34, color: "#bfdbfe"}}>
          --select get_/planets/&#123;planetId&#125; \
        </div>
        <div style={{paddingLeft: 34, color: "#bfdbfe"}}>
          --output planet.yaml
        </div>
        <div style={{height: 18}} />
        <div style={{color: "#86efac"}}>Wrote focused OpenAPI spec.</div>
      </Terminal>
      <div
        style={{
          ...panel,
          borderRadius: 18,
          padding: 26,
          fontSize: 24,
          lineHeight: 1.35,
        }}
      >
        <div style={{color: "#5eead4", fontWeight: 900, marginBottom: 16}}>
          planet.yaml
        </div>
        <div style={{...codeFont, color: "#cbd5e1"}}>
          <div>openapi: 3.1.0</div>
          <div>paths:</div>
          <div>  /planets/&#123;planetId&#125;:</div>
          <div style={{color: "#86efac"}}>    get:</div>
          <div>      summary: Get a planet</div>
          <div>components:</div>
          <div>  schemas:</div>
          <div style={{color: "#94a3b8"}}>    Planet: ...</div>
        </div>
      </div>
    </div>
  </Scene>
);

const OutroScene = () => (
  <Scene accent="#38bdf8">
    <div style={{display: "grid", gridTemplateColumns: "1fr 1fr", gap: 52}}>
      <div style={{alignSelf: "center"}}>
        <Eyebrow>AI-friendly API work</Eyebrow>
        <Headline>Smaller specs. Better prompts. Faster API work.</Headline>
        <Subline>
          Use it interactively in the terminal, or script it inside Codex,
          Claude, Cursor, and other agents.
        </Subline>
      </div>
      <div style={{alignSelf: "center"}}>
        <Terminal>
          <div>
            <span style={{color: "#5eead4"}}>$</span> go install
          </div>
          <div style={{color: "#bfdbfe"}}>
            github.com/korECM/openapi-extract@latest
          </div>
          <div style={{height: 20}} />
          <div>
            <span style={{color: "#5eead4"}}>$</span> openapi-extract tui
            openapi.yaml
          </div>
        </Terminal>
        <div
          style={{
            marginTop: 26,
            display: "grid",
            gridTemplateColumns: "1fr 1fr 1fr",
            gap: 14,
          }}
        >
          {["CLI", "TUI", "Agent skills"].map((item) => (
            <div
              key={item}
              style={{
                ...panel,
                borderRadius: 14,
                padding: "18px 20px",
                textAlign: "center",
                fontSize: 25,
                fontWeight: 800,
              }}
            >
              {item}
            </div>
          ))}
        </div>
      </div>
    </div>
  </Scene>
);

export const MyComposition = () => {
  const {fps} = useVideoConfig();

  return (
    <AbsoluteFill style={shell}>
      <Sequence durationInFrames={5 * fps} premountFor={fps}>
        <IntroScene />
      </Sequence>
      <Sequence from={5 * fps} durationInFrames={7 * fps} premountFor={fps}>
        <ProblemScene />
      </Sequence>
      <Sequence from={12 * fps} durationInFrames={6 * fps} premountFor={fps}>
        <ListScene />
      </Sequence>
      <Sequence from={18 * fps} durationInFrames={6 * fps} premountFor={fps}>
        <ExtractScene />
      </Sequence>
      <Sequence from={24 * fps} durationInFrames={6 * fps} premountFor={fps}>
        <OutroScene />
      </Sequence>
    </AbsoluteFill>
  );
};
