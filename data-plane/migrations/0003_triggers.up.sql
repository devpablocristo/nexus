CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_tools_set_updated_at ON tools;
CREATE TRIGGER trg_tools_set_updated_at
BEFORE UPDATE ON tools
FOR EACH ROW
EXECUTE PROCEDURE set_updated_at();

DROP TRIGGER IF EXISTS trg_policies_set_updated_at ON policies;
CREATE TRIGGER trg_policies_set_updated_at
BEFORE UPDATE ON policies
FOR EACH ROW
EXECUTE PROCEDURE set_updated_at();

