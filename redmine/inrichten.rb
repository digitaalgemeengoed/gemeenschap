# Redmine inrichten voor Digitaal GemeenGoed. Opnieuw draaien kan zonder schade.
#
#   docker compose -f compose.yml exec redmine \
#     bundle exec rails runner /usr/local/share/dg/inrichten.rb
#
# - instellingen: Nederlands, https://projecten.<domein>, REST-API aan;
# - inloggen via Keycloak (plugin redmine_oauth): een account ontstaat bij de
#   eerste login; het lokale formulier is verborgen (alleen voor het noodaccount);
# - het noodaccount "admin" met het wachtwoord en de API-sleutel uit .env
#   (de API-sleutel gebruikt de synchronisatie van de groepen);
# - rollen Gebruiker, Contributor, Member en Kerngroep, en de groepen
#   "Netwerk: …" met die rollen in het project "Office Assistent";
# - projectvoorstellen: het project "Projectvoorstellen" met de tracker
#   "Projectvoorstel"; een voorstel gaat naar de groep "Netwerk: Kerngroep".
#   Het account "site" (zonder beheerrechten, API-sleutel REDMINE_SITE_API_KEY)
#   maakt ze aan vanaf het formulier op de site; redmine_groepen.py maakt van
#   een goedgekeurd voorstel een project.

domein = ENV.fetch('DOMEIN')

# --- Instellingen ----------------------------------------------------------
{
  'app_title' => 'Projecten · Digitaal GemeenGoed',
  'host_name' => "projecten.#{domein}",
  'protocol' => 'https',
  'default_language' => 'nl',
  'force_default_language_for_anonymous' => '1',
  'rest_api_enabled' => '1',
  'login_required' => '0',
  'self_registration' => '0',
  'lost_password' => '0',
  'mail_from' => ENV['SMTP_AFZENDER'].presence || "noreply@#{domein}",
  'default_projects_public' => '1',
  'gravatar_enabled' => '0',
  'issue_group_assignment' => '1', # een melding of voorstel kan naar een groep (de kerngroep)
  'commit_ref_keywords' => 'refs,references,IssueID,melding',
  'commit_update_keywords' => [{ 'keywords' => 'fixes,closes,opgelost', 'status_id' => (IssueStatus.where(is_closed: true).first&.id).to_s }]
}.each { |k, v| Setting[k] = v }
puts 'Instellingen gezet'

# --- Inloggen via Keycloak --------------------------------------------------
Setting.plugin_redmine_oauth = (Setting.plugin_redmine_oauth || {}).merge(
  'hide_login_form' => '1',      # het formulier alleen achter "Inloggen met wachtwoord" (noodaccount)
  'self_registration' => '3',    # account aanmaken bij de eerste login, meteen actief
  'update_login' => '1',
  'oauth_logout' => '1',
  'oauth_login' => '0',
  'oauth_only_login' => '0'
)
provider = OauthProvider.find_or_initialize_by(custom_name: 'Digitaal GemeenGoed')
provider.assign_attributes(
  oauth_name: 'Keycloak',
  site: "https://aanmelden.#{domein}",
  tenant_id: 'gemeenschap',
  client_id: 'redmine',
  client_secret: ENV.fetch('REDMINE_CLIENT_SECRET'),
  identify_user_by: 'email',
  button_color: '#14478f',
  button_text: 'Inloggen met je DGG-account', # maximaal 40 tekens
  position: 1
)
provider.save!
puts 'Inloggen via Keycloak ingesteld'

# --- Noodaccount en API-sleutel ---------------------------------------------
admin = User.find_by(login: 'admin') or abort('Het account admin ontbreekt.')
admin.password = admin.password_confirmation = ENV.fetch('REDMINE_ADMIN_WACHTWOORD')
admin.must_change_passwd = false
admin.language = 'nl'
admin.save!
sleutel = Token.find_or_initialize_by(user_id: admin.id, action: 'api')
sleutel.value = ENV.fetch('REDMINE_API_KEY')
sleutel.save!
puts 'Noodaccount admin en API-sleutel gezet'

# --- Standaardgegevens (trackers, statussen, prioriteiten) ------------------
if Tracker.none?
  Redmine::DefaultData::Loader.load('nl')
  puts 'Standaardgegevens geladen (Nederlands)'
end

# --- Rollen ------------------------------------------------------------------
basis = %i[view_issues add_issues add_issue_notes view_issue_watchers view_wiki_pages view_messages add_messages
           view_news comment_news view_documents view_files view_calendar view_gantt view_time_entries]
rollen = {
  'Gebruiker' => basis,
  'Contributor' => basis + %i[edit_issues add_issue_watchers manage_subtasks edit_wiki_pages view_changesets browse_repository
                              view_wiki_edits log_time edit_own_messages],
  'Member' => basis + %i[edit_issues add_issue_watchers edit_wiki_pages view_changesets browse_repository view_wiki_edits
                         log_time edit_own_messages manage_versions save_queries],
  'Kerngroep' => basis + %i[edit_issues add_issue_watchers manage_subtasks edit_wiki_pages view_changesets browse_repository
                            view_wiki_edits log_time edit_own_messages manage_versions save_queries manage_categories
                            manage_boards edit_messages delete_messages manage_news manage_wiki manage_public_queries
                            set_issues_private delete_issues move_issues add_project manage_members],
  # Wie een project startte (vanuit een voorstel): beheert zijn eigen project.
  'Projectleider' => basis + %i[edit_issues add_issue_watchers manage_subtasks edit_wiki_pages view_changesets browse_repository
                                view_wiki_edits log_time edit_own_messages manage_versions save_queries manage_categories
                                manage_boards edit_messages delete_messages manage_news manage_wiki manage_public_queries
                                set_issues_private delete_issues edit_project select_project_modules manage_members],
  # Het account "site": alleen voorstellen indienen en lezen.
  'Site' => %i[view_issues add_issues]
}
rollen.each do |naam, rechten|
  r = Role.find_or_initialize_by(name: naam)
  r.permissions = rechten.select { |p| Redmine::AccessControl.permission(p) }
  r.issues_visibility = 'default'
  r.users_visibility = 'members_of_visible_projects' # ledenlijsten alleen binnen de eigen projecten
  r.assignable = !%w[Gebruiker Site].include?(naam)
  r.save!
end
puts "Rollen: #{rollen.keys.join(', ')}"

# --- Groepen en het eerste project ------------------------------------------
groepen = { 'Netwerk: Gebruikers' => 'Gebruiker', 'Netwerk: Contributors' => 'Contributor',
            'Netwerk: Members' => 'Member', 'Netwerk: Kerngroep' => 'Kerngroep' }
groepen.each_key { |n| Group.find_or_create_by!(lastname: n) }

# De fase van een project; het dashboard op de site toont die bij elke projectupdate.
fase = ProjectCustomField.find_or_initialize_by(name: 'Fase')
fase.field_format = 'list'
fase.possible_values = %w[Verkenning Ontwikkeling Pilot Beheer]
fase.visible = true
fase.is_filter = true
fase.save!

# De projecten van de site (inhoud/paginas/projecten.md). Wat beheerders in
# Redmine zelf aan een project veranderen, blijft staan: dit maakt alleen aan.
projecten = {
  'office-assistent' => ['Office Assistent', 'Metadata, anonimiseren en bewaren in Word, LibreOffice en Collabora.'],
  'xena' => ['XENA – Open DMS/RMA', 'Open document- en zaakbeheer voor gemeenten.'],
  'e-mailarchivering' => ['E-mailarchivering', 'Duurzame en veilige archivering van e-mail (VNG Selectielijst).'],
  'werkplek' => ['Object-native werkplek', 'WebDAV, CardDAV en CalDAV voor de overheid.'],
  'anonimiseren' => ['Anonimiseren bij de bron', 'Persoonsgegevens beschermen waar een stuk ontstaat.'],
  'open-overheid' => ['Open overheid', 'Van bronsysteem naar een zorgvuldig publicatieproces.'],
  'datafundament' => ['Datafundament', 'Een gedeelde basis voor gegevens, ETL en rapportage.']
}
projecten.each do |id, (naam, omschrijving)|
  project = Project.find_by(identifier: id) || Project.create!(
    name: naam, identifier: id, is_public: true, description: omschrijving,
    enabled_module_names: %w[issue_tracking boards news files calendar gantt roadmap]
  )
  # Kennis staat in de kennisbank (kennis.<domein>), niet in een wiki per project.
  # Een projectwiki waarin al iets staat, blijft.
  if project.module_enabled?(:wiki) && (project.wiki.nil? || project.wiki.pages.empty?)
    project.disable_module!(:wiki)
  end
  project.trackers = Tracker.all if project.trackers.empty?
  Board.find_or_create_by!(project: project, name: 'Algemeen') { |b| b.description = 'Vragen en ideeën over dit project.' }
  groepen.each do |groep, rol|
    g = Group.find_by(lastname: groep)
    m = Member.find_or_initialize_by(project: project, principal: g)
    m.role_ids = [Role.find_by(name: rol).id]
    m.save!
  end
end
puts "Projecten: #{projecten.values.map(&:first).join(', ')}, met de netwerkgroepen en het veld Fase"

# --- Projectvoorstellen -------------------------------------------------------
# Een lid dient op de site een voorstel in (formulier "Start een project"). Dat
# wordt een issue in het project Projectvoorstellen, toegewezen aan de
# kerngroep. Zet de kerngroep het op Goedgekeurd, dan maakt redmine_groepen.py
# het project aan (met de indiener als projectleider) en zet het op Gestart.
status = ->(naam, gesloten) do
  st = IssueStatus.find_or_initialize_by(name: naam)
  st.is_closed = gesloten
  st.save!
  st
end
nieuw = IssueStatus.where(is_closed: false).order(:position).first
in_bespreking = status.('In bespreking', false)
goedgekeurd = status.('Goedgekeurd', false)
gestart = status.('Gestart', true)
afgewezen = status.('Afgewezen', true)

voorstel = Tracker.find_or_initialize_by(name: 'Projectvoorstel')
voorstel.default_status = nieuw
voorstel.description = 'Een nieuw project, voorgesteld door een lid (formulier op de site).'
voorstel.core_fields = %w[assigned_to_id description]
voorstel.save!
# Andere projecten krijgen deze tracker niet.
Project.where.not(identifier: 'projectvoorstellen').find_each { |p| p.trackers.delete(voorstel) if p.trackers.include?(voorstel) }

# Wie mag een voorstel van status veranderen: de kerngroep (en de beheerders).
overgangen = {
  nieuw => [in_bespreking, goedgekeurd, afgewezen],
  in_bespreking => [nieuw, goedgekeurd, afgewezen],
  goedgekeurd => [in_bespreking, gestart, afgewezen],
  afgewezen => [in_bespreking]
}
kerngroep_rol = Role.find_by!(name: 'Kerngroep')
overgangen.each do |van, naar|
  naar.each do |n|
    WorkflowTransition.find_or_create_by!(tracker: voorstel, role: kerngroep_rol, old_status: van, new_status: n,
                                          author: false, assignee: false)
  end
end

velden = {
  'Mapnaam' => { format: 'string', regexp: '^[a-z][a-z0-9-]{2,39}$', verplicht: true,
                 uitleg: 'Wordt het adres van het project (projecten.…/projects/<mapnaam>) en de map in Documenten. Ligt vast zodra het project start.' },
  'Indiener' => { format: 'string', regexp: '', verplicht: true, uitleg: 'E-mailadres van wie het voorstel indiende; wordt projectleider.' },
  'Gemeenten' => { format: 'string', regexp: '', verplicht: false, uitleg: 'Gemeenten en organisaties die (willen) meedoen.' }
}
cf = velden.to_h do |naam, v|
  f = IssueCustomField.find_or_initialize_by(name: naam)
  f.field_format = v[:format]
  f.regexp = v[:regexp]
  f.is_required = v[:verplicht]
  f.is_filter = true
  f.searchable = true
  f.visible = true
  f.is_for_all = false
  f.description = v[:uitleg]
  f.trackers = [voorstel]
  f.save!
  [naam, f]
end

pv = Project.find_by(identifier: 'projectvoorstellen') || Project.create!(
  name: 'Projectvoorstellen', identifier: 'projectvoorstellen', is_public: false,
  description: 'Voorstellen voor nieuwe projecten, ingediend via "Start een project" op de site. De kerngroep beoordeelt ze.',
  enabled_module_names: %w[issue_tracking]
)
pv.trackers = [voorstel]
pv.issue_custom_fields = cf.values
groepen.each do |groep, rol|
  m = Member.find_or_initialize_by(project: pv, principal: Group.find_by(lastname: groep))
  m.role_ids = [Role.find_by(name: rol).id]
  m.save!
end
pv.default_assigned_to = Group.find_by(lastname: 'Netwerk: Kerngroep')
pv.save!

# Het account "site": geen beheerder, alleen voorstellen indienen.
site = User.find_by(login: 'site') || User.new(login: 'site', firstname: 'Website', lastname: 'Digitaal GemeenGoed',
                                                mail: "site@#{domein}", language: 'nl')
site.admin = false
site.status = User::STATUS_ACTIVE
site.mail_notification = 'none'
if site.new_record?
  site.password = site.password_confirmation = SecureRandom.hex(24)
  site.must_change_passwd = false
end
site.save!
site_sleutel = Token.find_or_initialize_by(user_id: site.id, action: 'api')
site_sleutel.value = ENV.fetch('REDMINE_SITE_API_KEY')
site_sleutel.save!
m = Member.find_or_initialize_by(project: pv, principal: site)
m.role_ids = [Role.find_by(name: 'Site').id]
m.save!
puts 'Projectvoorstellen ingericht (tracker, statussen, velden, kerngroep, account site)'
puts 'Redmine ingericht'
