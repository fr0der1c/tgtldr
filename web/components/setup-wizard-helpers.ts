import { Bootstrap, PendingAuth } from "@/lib/types";
import {
  LoginStage,
  setupSteps,
  SetupStep
} from "@/components/setup-wizard-types";

export function resolveCurrentStep(
  bootstrap: Bootstrap,
  current: SetupStep
): SetupStep {
  if (current === "bot" && bootstrap.telegramAuthorized) {
    return "bot";
  }
  if (current === "login" && bootstrap.settingsConfigured) {
    return bootstrap.telegramAuthorized ? "bot" : "login";
  }
  if (!bootstrap.settingsConfigured) {
    return "config";
  }
  if (!bootstrap.telegramAuthorized) {
    return "login";
  }
  return "bot";
}

export function resolveLoginStage(
  telegramAuthorized: boolean,
  pendingAuth: PendingAuth | null,
  override: LoginStage | null
): LoginStage {
  if (override) {
    return override;
  }
  if (telegramAuthorized) {
    return "success";
  }
  if (pendingAuth?.step === "password") {
    return "password";
  }
  if (pendingAuth?.step === "code") {
    return "code";
  }
  return "phone";
}

export function stepEnabled(step: SetupStep, bootstrap: Bootstrap | null) {
  if (step === "config") {
    return true;
  }
  if (step === "login") {
    return bootstrap?.settingsConfigured ?? false;
  }
  return bootstrap?.telegramAuthorized ?? false;
}

export function stepState(
  step: SetupStep,
  currentStep: SetupStep,
  bootstrap: Bootstrap | null
) {
  if (!stepEnabled(step, bootstrap)) {
    return "locked";
  }
  if (stepIndex(step) < stepIndex(currentStep)) {
    return "completed";
  }
  if (step === currentStep) {
    return "active";
  }
  return "upcoming";
}

export function stepIndex(step: SetupStep) {
  return setupSteps.findIndex((item) => item.key === step);
}

export function asMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}
