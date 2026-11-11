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
#   "Netwerk: …" met die rollen in het project "Office Assistent".

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
                            set_issues_private delete_issues move_issues add_project manage_members]
}
rollen.each do |naam, rechten|
  r = Role.find_or_initialize_by(name: naam)
  r.permissions = rechten.select { |p| Redmine::AccessControl.permission(p) }
  r.issues_visibility = 'default'
  r.users_visibility = 'members_of_visible_projects' # ledenlijsten alleen binnen de eigen projecten
  r.assignable = naam != 'Gebruiker'
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
puts 'Redmine ingericht'
