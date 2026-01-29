"use client";

import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { AppShell } from "../../components/app-shell";
import { RequireAuth } from "../../components/require-auth";
import { SectionHeader } from "../../components/section-header";
import {
  apiCreateService,
  apiCreateStaff,
  apiGetBusinessProfile,
  apiListServices,
  apiListStaff,
  apiUpdateBusinessProfile
} from "../../lib/api";

const steps = [
  { value: "profile", label: "Business profile" },
  { value: "services", label: "Services" },
  { value: "staff", label: "Staff" }
];

function parseOffsets(input: string): number[] {
  return input
    .split(",")
    .map((value) => Number(value.trim()))
    .filter((value) => Number.isFinite(value) && value >= 0);
}

export default function OnboardingPage() {
  const [step, setStep] = useState<(typeof steps)[number]["value"]>("profile");
  const stepIndex = Math.max(
    0,
    steps.findIndex((item) => item.value === step)
  );

  const [profileLoading, setProfileLoading] = useState(false);
  const [profileError, setProfileError] = useState<string | null>(null);
  const [profileName, setProfileName] = useState("");
  const [profileTimezone, setProfileTimezone] = useState("UTC");
  const [profileOffsets, setProfileOffsets] = useState("1440, 60");

  const [servicesLoading, setServicesLoading] = useState(false);
  const [servicesError, setServicesError] = useState<string | null>(null);
  const [services, setServices] = useState<
    Array<{
      id: string;
      name: string;
      duration_minutes: number;
      price: string;
      description?: string;
    }>
  >([]);
  const [serviceName, setServiceName] = useState("");
  const [serviceDuration, setServiceDuration] = useState("30");
  const [servicePrice, setServicePrice] = useState("25");
  const [serviceDescription, setServiceDescription] = useState("");

  const [staffLoading, setStaffLoading] = useState(false);
  const [staffError, setStaffError] = useState<string | null>(null);
  const [staff, setStaff] = useState<
    Array<{ id: string; name: string; is_active: boolean }>
  >([]);
  const [staffName, setStaffName] = useState("");
  const [staffActive, setStaffActive] = useState(true);

  useEffect(() => {
    let ignore = false;
    async function loadProfile() {
      setProfileLoading(true);
      setProfileError(null);
      try {
        const profile = await apiGetBusinessProfile();
        if (ignore) return;
        setProfileName(profile.name ?? "");
        setProfileTimezone(profile.timezone ?? "UTC");
        setProfileOffsets(
          (profile.reminder_offsets_minutes ?? [1440, 60]).join(", ")
        );
      } catch (err) {
        if (!ignore) {
          setProfileError("Unable to load profile");
        }
      } finally {
        if (!ignore) {
          setProfileLoading(false);
        }
      }
    }

    loadProfile();
    return () => {
      ignore = true;
    };
  }, []);

  useEffect(() => {
    let ignore = false;
    async function loadServices() {
      setServicesLoading(true);
      setServicesError(null);
      try {
        const list = await apiListServices();
        if (!ignore) {
          setServices(Array.isArray(list) ? list : []);
        }
      } catch (err) {
        if (!ignore) {
          setServicesError("Unable to load services");
        }
      } finally {
        if (!ignore) {
          setServicesLoading(false);
        }
      }
    }

    loadServices();
    return () => {
      ignore = true;
    };
  }, []);

  useEffect(() => {
    let ignore = false;
    async function loadStaff() {
      setStaffLoading(true);
      setStaffError(null);
      try {
        const list = await apiListStaff();
        if (!ignore) {
          setStaff(Array.isArray(list) ? list : []);
        }
      } catch (err) {
        if (!ignore) {
          setStaffError("Unable to load staff");
        }
      } finally {
        if (!ignore) {
          setStaffLoading(false);
        }
      }
    }

    loadStaff();
    return () => {
      ignore = true;
    };
  }, []);

  async function onSaveProfile() {
    setProfileLoading(true);
    setProfileError(null);
    try {
      await apiUpdateBusinessProfile({
        name: profileName,
        timezone: profileTimezone,
        reminder_offsets_minutes: parseOffsets(profileOffsets)
      });
    } catch (err) {
      setProfileError("Unable to update profile");
    } finally {
      setProfileLoading(false);
    }
  }

  async function onCreateService(e: React.FormEvent) {
    e.preventDefault();
    setServicesLoading(true);
    setServicesError(null);
    try {
      await apiCreateService({
        name: serviceName,
        duration_minutes: Number(serviceDuration),
        price: Number(servicePrice),
        description: serviceDescription || undefined
      });
      const list = await apiListServices();
      setServices(Array.isArray(list) ? list : []);
      setServiceName("");
      setServiceDuration("30");
      setServicePrice("25");
      setServiceDescription("");
    } catch (err) {
      setServicesError("Unable to create service");
    } finally {
      setServicesLoading(false);
    }
  }

  async function onCreateStaff(e: React.FormEvent) {
    e.preventDefault();
    setStaffLoading(true);
    setStaffError(null);
    try {
      await apiCreateStaff({ name: staffName, is_active: staffActive });
      const list = await apiListStaff();
      setStaff(Array.isArray(list) ? list : []);
      setStaffName("");
      setStaffActive(true);
    } catch (err) {
      setStaffError("Unable to create staff member");
    } finally {
      setStaffLoading(false);
    }
  }

  return (
    <RequireAuth>
      <AppShell>
        <SectionHeader
          title="Owner onboarding"
          description="Complete the three setup steps to start taking bookings."
          action={
            <Badge variant="secondary">
              Step {stepIndex + 1} of {steps.length}
            </Badge>
          }
        />

        <Tabs
          value={step}
          onValueChange={(value) =>
            setStep(value as (typeof steps)[number]["value"])
          }
          className="mt-6"
        >
          <TabsList className="grid w-full grid-cols-3">
            {steps.map((item) => (
              <TabsTrigger key={item.value} value={item.value}>
                {item.label}
              </TabsTrigger>
            ))}
          </TabsList>

          <TabsContent value="profile" className="mt-6">
            <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
            <Card>
              <CardHeader>
                <CardTitle>Business profile</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="profile-name">Business name</Label>
                  <Input
                    id="profile-name"
                    value={profileName}
                    onChange={(e) => setProfileName(e.target.value)}
                    placeholder="Demo Salon"
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="profile-timezone">Timezone</Label>
                  <Input
                    id="profile-timezone"
                    value={profileTimezone}
                    onChange={(e) => setProfileTimezone(e.target.value)}
                    placeholder="UTC"
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="profile-reminders">
                    Reminder offsets (minutes)
                  </Label>
                  <Input
                    id="profile-reminders"
                    value={profileOffsets}
                    onChange={(e) => setProfileOffsets(e.target.value)}
                    placeholder="1440, 60"
                  />
                  <p className="text-xs text-muted-foreground">
                    Comma-separated. Example: 1440, 60
                  </p>
                </div>
                {profileError ? (
                  <p className="text-sm text-destructive">{profileError}</p>
                ) : null}
                <Button onClick={onSaveProfile} disabled={profileLoading}>
                  {profileLoading ? "Saving..." : "Save profile"}
                </Button>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Why this matters</CardTitle>
              </CardHeader>
              <CardContent className="text-sm text-muted-foreground">
                Profile settings control the public booking experience, including
                timezone and reminder cadence.
              </CardContent>
            </Card>
            </div>
          </TabsContent>

          <TabsContent value="services" className="mt-6">
            <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
              <div className="grid gap-4">
                <Card>
                  <CardHeader>
                    <CardTitle>Service catalog</CardTitle>
                    <p className="text-sm text-muted-foreground">
                      Define the services your customers can book.
                    </p>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <form className="grid gap-3" onSubmit={onCreateService}>
                      <div className="grid gap-3 md:grid-cols-2">
                        <div className="space-y-2">
                          <Label htmlFor="service-name">Service name</Label>
                          <Input
                            id="service-name"
                            value={serviceName}
                            onChange={(e) => setServiceName(e.target.value)}
                            placeholder="Consultation"
                            required
                          />
                        </div>
                        <div className="space-y-2">
                          <Label htmlFor="service-duration">Duration (min)</Label>
                          <Input
                            id="service-duration"
                            type="number"
                            value={serviceDuration}
                            onChange={(e) => setServiceDuration(e.target.value)}
                            min={5}
                            required
                          />
                        </div>
                      </div>
                      <div className="grid gap-3 md:grid-cols-2">
                        <div className="space-y-2">
                          <Label htmlFor="service-price">Price</Label>
                          <Input
                            id="service-price"
                            type="number"
                            value={servicePrice}
                            onChange={(e) => setServicePrice(e.target.value)}
                            min={0}
                            step="0.01"
                            required
                          />
                        </div>
                        <div className="space-y-2">
                          <Label htmlFor="service-description">Description</Label>
                          <Input
                            id="service-description"
                            value={serviceDescription}
                            onChange={(e) => setServiceDescription(e.target.value)}
                            placeholder="Initial consult"
                          />
                        </div>
                      </div>
                      {servicesError ? (
                        <p className="text-sm text-destructive">{servicesError}</p>
                      ) : null}
                      <div className="flex items-center gap-3">
                        <Button type="submit" disabled={servicesLoading}>
                          {servicesLoading ? "Adding..." : "Add service"}
                        </Button>
                        <span className="text-xs text-muted-foreground">
                          Typical: 30–60 mins
                        </span>
                      </div>
                    </form>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader>
                    <CardTitle>Active services</CardTitle>
                    <p className="text-sm text-muted-foreground">
                      These show up on your public booking page.
                    </p>
                  </CardHeader>
                  <CardContent className="grid gap-3 md:grid-cols-2">
                    {(services ?? []).length === 0 ? (
                      <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
                        No services yet. Add at least one to continue.
                      </div>
                    ) : null}
                    {(services ?? []).map((service) => (
                      <Card key={service.id} data-testid="service-card">
                        <CardHeader>
                          <div className="flex items-start justify-between gap-2">
                            <CardTitle className="text-base">
                              {service.name || "Untitled service"}
                            </CardTitle>
                            <Badge variant="outline">
                              {service.duration_minutes}m
                            </Badge>
                          </div>
                        </CardHeader>
                        <CardContent className="text-sm text-muted-foreground">
                          <p>{service.description ?? "No description"}</p>
                          <div className="mt-2 flex items-center gap-2 text-xs">
                            <Badge variant="secondary">${service.price}</Badge>
                            <span>Public booking</span>
                          </div>
                        </CardContent>
                      </Card>
                    ))}
                  </CardContent>
                </Card>
              </div>

              <Card>
                <CardHeader>
                  <CardTitle>Setup tip</CardTitle>
                </CardHeader>
                <CardContent className="space-y-3 text-sm text-muted-foreground">
                  <p>
                    Start with 1–3 services customers book most often. You can
                    expand later.
                  </p>
                  <div className="space-y-2">
                    <Badge variant="secondary">Popular</Badge>
                    <p>Consultation · Haircut · Checkup</p>
                  </div>
                </CardContent>
              </Card>
            </div>
          </TabsContent>

          <TabsContent value="staff" className="mt-6">
            <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
              <div className="grid gap-4">
                <Card>
                  <CardHeader>
                    <CardTitle>Team members</CardTitle>
                    <p className="text-sm text-muted-foreground">
                      Add the staff who will take appointments.
                    </p>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <form className="grid gap-3" onSubmit={onCreateStaff}>
                      <div className="space-y-2">
                        <Label htmlFor="staff-name">Staff name</Label>
                        <Input
                          id="staff-name"
                          value={staffName}
                          onChange={(e) => setStaffName(e.target.value)}
                          placeholder="Alex"
                          required
                        />
                      </div>
                      <label className="flex items-center gap-2 text-sm text-muted-foreground">
                        <input
                          type="checkbox"
                          checked={staffActive}
                          onChange={(e) => setStaffActive(e.target.checked)}
                        />
                        Active (accepting bookings)
                      </label>
                      {staffError ? (
                        <p className="text-sm text-destructive">{staffError}</p>
                      ) : null}
                      <Button type="submit" disabled={staffLoading}>
                        {staffLoading ? "Adding..." : "Add staff"}
                      </Button>
                    </form>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader>
                    <CardTitle>Active staff</CardTitle>
                    <p className="text-sm text-muted-foreground">
                      Configure schedules after adding staff.
                    </p>
                  </CardHeader>
                  <CardContent className="grid gap-3 md:grid-cols-2">
                    {(staff ?? []).length === 0 ? (
                      <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
                        No staff yet. Add at least one to open bookings.
                      </div>
                    ) : null}
                    {(staff ?? []).map((member) => (
                      <Card key={member.id} data-testid="staff-card">
                        <CardHeader>
                          <div className="flex items-center justify-between gap-2">
                            <CardTitle className="text-base">
                              {member.name?.trim() || "Unnamed staff"}
                            </CardTitle>
                            <Badge variant={member.is_active ? "secondary" : "outline"}>
                              {member.is_active ? "Active" : "Inactive"}
                            </Badge>
                          </div>
                        </CardHeader>
                        <CardContent className="text-xs text-muted-foreground">
                          Status: {member.is_active ? "Active" : "Inactive"}
                        </CardContent>
                      </Card>
                    ))}
                  </CardContent>
                </Card>
              </div>

              <Card>
                <CardHeader>
                  <CardTitle>Next up</CardTitle>
                </CardHeader>
                <CardContent className="space-y-3 text-sm text-muted-foreground">
                  <p>
                    After staff is set, configure working hours and time-off in
                    the Availability section.
                  </p>
                  <Badge variant="outline">Availability</Badge>
                </CardContent>
              </Card>
            </div>
          </TabsContent>
        </Tabs>
      </AppShell>
    </RequireAuth>
  );
}
