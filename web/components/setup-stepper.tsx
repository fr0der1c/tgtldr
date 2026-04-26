"use client";

import { Bootstrap } from "@/lib/types";
import { Card, StatusPill } from "@/components/ui";
import {
  setupSteps,
  SetupStep
} from "@/components/setup-wizard-types";
import {
  stepEnabled,
  stepIndex,
  stepState
} from "@/components/setup-wizard-helpers";

type SetupStepperProps = {
  bootstrap: Bootstrap | null;
  currentStep: SetupStep;
  onStepChange: (step: SetupStep) => void;
};

export function SetupStepper({
  bootstrap,
  currentStep,
  onStepChange
}: SetupStepperProps) {
  return (
    <Card>
      <div className="setup-progress">
        <div className="setup-progress-head">
          <div>
            <p className="eyebrow">TGTLDR</p>
            <h2 className="setup-progress-title">欢迎使用，请先完成设置向导</h2>
          </div>
          <StatusPill tone={bootstrap?.telegramAuthorized ? "good" : "warn"}>
            步骤 {stepIndex(currentStep) + 1}/{setupSteps.length}
          </StatusPill>
        </div>
        <div className="setup-progress-track">
          <div
            className="setup-progress-fill"
            style={{ width: `${((stepIndex(currentStep) + 1) / setupSteps.length) * 100}%` }}
          />
        </div>
        <div className="setup-step-grid">
          {setupSteps.map((step) => {
            const state = stepState(step.key, currentStep, bootstrap);
            return (
              <button
                key={step.key}
                className={`setup-step ${state}`}
                disabled={!stepEnabled(step.key, bootstrap)}
                onClick={() => onStepChange(step.key)}
                type="button"
              >
                <span className="setup-step-index">{stepIndex(step.key) + 1}</span>
                <span className="setup-step-meta">
                  <span className="setup-step-eyebrow">{step.eyebrow}</span>
                  <strong>{step.label}</strong>
                </span>
              </button>
            );
          })}
        </div>
      </div>
    </Card>
  );
}
